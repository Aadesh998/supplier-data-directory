package store

import (
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ExportCSV produces a permission-filtered CSV. Each field is included only if its provenance
// permits the requested purpose. Personal (individual) contacts are never exported.
//   - internal: non-deleted companies, fields from sources allowing internal use
//   - public: published companies, publicly permitted fields (same as the public API)
//   - commercial: companies not withdrawn from commercial use whose display name is commercially
//     permitted; every other field individually checked against commercial rights
func (s *Store) ExportCSV(ctx context.Context, a Actor, purpose Purpose, publicationStatus string) ([]byte, int, error) {
	var base string
	switch purpose {
	case PurposePublic:
		base = publishedWhere
	case PurposeCommercial:
		base = `c.deleted_at IS NULL AND c.commercial_api_withdrawn_at IS NULL AND EXISTS (SELECT 1 FROM commercial_field_permissions cp
			WHERE cp.company_id = c.id AND cp.entity_type = 'company' AND cp.entity_id = c.id AND cp.field_name = 'display_name')`
	default:
		base = `c.deleted_at IS NULL AND EXISTS (SELECT 1 FROM internal_field_permissions ip
			WHERE ip.company_id = c.id AND ip.entity_type = 'company' AND ip.entity_id = c.id AND ip.field_name = 'display_name')`
	}
	args := []any{}
	if publicationStatus != "" {
		base += " AND c.publication_status = $1"
		args = append(args, publicationStatus)
	}
	raws, err := collect(ctx, s.Pool, `SELECT c.id, c.slug, c.display_name, c.verification_status, c.legal_name, c.description,
		c.website_url, c.establishment_year, c.updated_at, c.published_at, c.last_verified_at FROM companies c WHERE `+base+
		` ORDER BY c.display_name`, scanRaw, args...)
	if err != nil {
		return nil, 0, err
	}
	statuses := map[string]string{}
	if purpose == PurposeInternal {
		rows, err := collect(ctx, s.Pool, `SELECT id, publication_status FROM companies WHERE deleted_at IS NULL`, func(r pgx.Rows) ([2]string, error) {
			var x [2]string
			return x, r.Scan(&x[0], &x[1])
		})
		if err != nil {
			return nil, 0, err
		}
		for _, r := range rows {
			statuses[r[0]] = r[1]
		}
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	header := []string{"slug", "display_name", "legal_name", "description", "website_url", "establishment_year", "roles",
		"categories", "city", "state_code", "products", "business_contacts", "verification_status", "updated_at"}
	if purpose == PurposeInternal {
		header = append(header, "publication_status")
	}
	_ = w.Write(header)
	for _, r := range raws {
		pc, err := buildPublicCompany(ctx, s.Pool, purpose, r)
		if err != nil {
			return nil, 0, err
		}
		cats := make([]string, len(pc.Categories))
		for i, c := range pc.Categories {
			cats[i] = c.Slug
		}
		prods := make([]string, len(pc.Products))
		for i, p := range pc.Products {
			prods[i] = p.Name
		}
		contacts := make([]string, len(pc.Contacts))
		for i, c := range pc.Contacts {
			contacts[i] = c.ContactType + ":" + c.Value
		}
		year := ""
		if pc.EstablishmentYear != 0 {
			year = strconv.Itoa(pc.EstablishmentYear)
		}
		rec := []string{pc.Slug, pc.DisplayName, pc.LegalName, pc.Description, pc.WebsiteURL, year, strings.Join(pc.Roles, ";"),
			strings.Join(cats, ";"), pc.City, pc.StateCode, strings.Join(prods, ";"), strings.Join(contacts, ";"),
			pc.VerificationStatus, pc.UpdatedAt.Format(time.RFC3339)}
		if purpose == PurposeInternal {
			rec = append(rec, statuses[r.id])
		}
		for i := range rec {
			rec[i] = safeCSV(rec[i])
		}
		_ = w.Write(rec)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, 0, err
	}
	err = Audit(ctx, s.Pool, a, "export.create", "export", nil, nil, map[string]any{"scope": string(purpose), "rows": len(raws),
		"publication_status": publicationStatus})
	return buf.Bytes(), len(raws), err
}
