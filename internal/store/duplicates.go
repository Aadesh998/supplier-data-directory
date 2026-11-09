package store

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// CandidateKey describes a record to match against existing companies.
type CandidateKey struct {
	ExcludeID      string
	NormalizedName string
	Domain         string
	City           string
	GSTIN          string
	ContactHashes  []string
}

// Candidate is a possible duplicate with an explainable score.
type Candidate struct {
	CompanyID   string   `json:"company_id"`
	DisplayName string   `json:"display_name"`
	Score       float64  `json:"score"`
	Reasons     []string `json:"reasons"`
}

// DuplicateThreshold is the minimum score for a pair to be surfaced for review.
const DuplicateThreshold = 0.6

// FindCandidates returns likely duplicates. Matching never merges anything by itself.
func FindCandidates(ctx context.Context, q DBTX, k CandidateKey) ([]Candidate, error) {
	if k.NormalizedName == "" && k.Domain == "" && k.GSTIN == "" && len(k.ContactHashes) == 0 {
		return nil, nil
	}
	type row struct {
		id, name             string
		sim                  float64
		sameDomain, sameCont bool
		sameCity, sameGSTIN  bool
	}
	rows, err := collect(ctx, q, `SELECT o.id, o.display_name,
			similarity(o.normalized_name, $1) AS sim,
			COALESCE($2 <> '' AND o.website_domain = $2, false) AS same_domain,
			EXISTS (SELECT 1 FROM contacts y WHERE y.company_id = o.id AND y.deleted_at IS NULL AND y.value_hash = ANY($3)) AS same_contact,
			($4 <> '' AND EXISTS (SELECT 1 FROM facilities fb WHERE fb.company_id = o.id AND fb.deleted_at IS NULL AND lower(fb.city) = lower($4))) AS same_city,
			COALESCE($6 <> '' AND o.gstin = $6, false) AS same_gstin
		FROM companies o
		WHERE o.deleted_at IS NULL AND ($5 = '' OR o.id <> $5::uuid)
		  AND (($1 <> '' AND o.normalized_name % $1)
		       OR ($2 <> '' AND o.website_domain = $2)
		       OR ($6 <> '' AND o.gstin = $6)
		       OR EXISTS (SELECT 1 FROM contacts y WHERE y.company_id = o.id AND y.deleted_at IS NULL AND y.value_hash = ANY($3)))
		ORDER BY sim DESC LIMIT 10`, func(r pgx.Rows) (row, error) {
		var x row
		return x, r.Scan(&x.id, &x.name, &x.sim, &x.sameDomain, &x.sameCont, &x.sameCity, &x.sameGSTIN)
	}, k.NormalizedName, k.Domain, k.ContactHashes, k.City, k.ExcludeID, k.GSTIN)
	if err != nil {
		return nil, err
	}
	var out []Candidate
	for _, r := range rows {
		score := r.sim
		reasons := []string{}
		if r.sim >= 0.45 {
			reasons = append(reasons, fmt.Sprintf("similar name (%.2f)", r.sim))
		}
		if r.sameDomain {
			score = math.Max(score, 0.95)
			reasons = append(reasons, "same website domain")
		}
		if r.sameGSTIN {
			score = math.Max(score, 0.98)
			reasons = append(reasons, "same GSTIN")
		}
		if r.sameCont {
			score = math.Max(score, 0.9)
			reasons = append(reasons, "same business phone or email")
		}
		if r.sameCity && r.sim >= 0.4 {
			score += 0.1
			reasons = append(reasons, "same city")
		}
		score = math.Min(1, math.Round(score*1000)/1000)
		if score >= DuplicateThreshold {
			out = append(out, Candidate{CompanyID: r.id, DisplayName: r.name, Score: score, Reasons: reasons})
		}
	}
	return out, nil
}

func companyKey(ctx context.Context, q DBTX, id string) (CandidateKey, error) {
	k := CandidateKey{ExcludeID: id}
	var domain, city, gstin *string
	err := q.QueryRow(ctx, `SELECT c.normalized_name, c.website_domain, c.gstin,
		(SELECT city FROM facilities WHERE company_id = c.id AND deleted_at IS NULL ORDER BY is_primary DESC LIMIT 1),
		COALESCE((SELECT array_agg(value_hash) FROM contacts WHERE company_id = c.id AND deleted_at IS NULL AND contact_type <> 'individual'), '{}')
		FROM companies c WHERE c.id = $1`, id).Scan(&k.NormalizedName, &domain, &gstin, &city, &k.ContactHashes)
	k.Domain, k.City, k.GSTIN = deref(domain), deref(city), deref(gstin)
	return k, err
}

// FindDuplicatesFor records open duplicate pairs for one company; returns the number created.
func FindDuplicatesFor(ctx context.Context, q DBTX, companyID string) (int, error) {
	k, err := companyKey(ctx, q, companyID)
	if err != nil {
		return 0, err
	}
	cands, err := FindCandidates(ctx, q, k)
	if err != nil {
		return 0, err
	}
	created := 0
	for _, c := range cands {
		a, b := companyID, c.CompanyID
		if b < a {
			a, b = b, a
		}
		tag, err := q.Exec(ctx, `INSERT INTO duplicate_pairs (company_a_id, company_b_id, score, reasons)
			VALUES ($1, $2, $3, $4) ON CONFLICT (company_a_id, company_b_id) DO NOTHING`, a, b, c.Score, c.Reasons)
		if err != nil {
			return created, err
		}
		created += int(tag.RowsAffected())
	}
	return created, nil
}

// ScanDuplicates checks every active company and records new candidate pairs.
func (s *Store) ScanDuplicates(ctx context.Context, a Actor) (int, error) {
	ids, err := collect(ctx, s.Pool, `SELECT id FROM companies WHERE deleted_at IS NULL ORDER BY created_at`, func(r pgx.Rows) (string, error) {
		var x string
		return x, r.Scan(&x)
	})
	if err != nil {
		return 0, err
	}
	total := 0
	for _, id := range ids {
		n, err := FindDuplicatesFor(ctx, s.Pool, id)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, Audit(ctx, s.Pool, a, "duplicates.scan", "duplicate_pair", nil, nil, map[string]any{"created": total})
}

// DuplicatePair is a reviewable pair.
type DuplicatePair struct {
	ID        string              `json:"id"`
	Score     float64             `json:"score"`
	Reasons   []string            `json:"reasons"`
	Status    string              `json:"status"`
	CompanyA  AdminCompanySummary `json:"company_a"`
	CompanyB  AdminCompanySummary `json:"company_b"`
	CreatedAt time.Time           `json:"created_at"`
}

// GetDuplicatePair loads one pair with both summaries.
func GetDuplicatePair(ctx context.Context, q DBTX, id string) (DuplicatePair, error) {
	var d DuplicatePair
	var a, b string
	if err := q.QueryRow(ctx, `SELECT id, score::float8, reasons, status, company_a_id, company_b_id, created_at FROM duplicate_pairs WHERE id = $1`, id).
		Scan(&d.ID, &d.Score, &d.Reasons, &d.Status, &a, &b, &d.CreatedAt); err != nil {
		return d, err
	}
	var err error
	if d.CompanyA, err = scanSummary(q.QueryRow(ctx, summarySelect+" WHERE c.id = $1", a)); err != nil {
		return d, err
	}
	d.CompanyB, err = scanSummary(q.QueryRow(ctx, summarySelect+" WHERE c.id = $1", b))
	return d, err
}

// ListDuplicatePairs lists pairs by status, highest score first.
func ListDuplicatePairs(ctx context.Context, q DBTX, status string, limit, offset int) ([]DuplicatePair, int, error) {
	var total int
	if err := q.QueryRow(ctx, `SELECT count(*) FROM duplicate_pairs WHERE status = $1`, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	ids, err := collect(ctx, q, `SELECT id FROM duplicate_pairs WHERE status = $1 ORDER BY score DESC, created_at LIMIT $2 OFFSET $3`,
		func(r pgx.Rows) (string, error) {
			var x string
			return x, r.Scan(&x)
		}, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	out := make([]DuplicatePair, 0, len(ids))
	for _, id := range ids {
		d, err := GetDuplicatePair(ctx, q, id)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, d)
	}
	return out, total, nil
}

// ResolveDuplicate dismisses a pair or merges one company into the other.
func (s *Store) ResolveDuplicate(ctx context.Context, a Actor, id, action, keep, notes string) error {
	return s.InTx(ctx, func(tx pgx.Tx) error {
		var status, ca, cb string
		if err := tx.QueryRow(ctx, `SELECT status, company_a_id, company_b_id FROM duplicate_pairs WHERE id = $1 FOR UPDATE`, id).Scan(&status, &ca, &cb); err != nil {
			return err
		}
		if status != "open" {
			return ErrConflict{Msg: "duplicate pair is already " + status}
		}
		newStatus := "dismissed"
		if action == "merge" {
			keepID, removeID := ca, cb
			if keep == "b" {
				keepID, removeID = cb, ca
			}
			if err := MergeCompanies(ctx, tx, a, keepID, removeID); err != nil {
				return err
			}
			newStatus = "merged"
		}
		if _, err := tx.Exec(ctx, `UPDATE duplicate_pairs SET status = $2, resolved_by = $3, resolved_at = now(), resolution_notes = NULLIF($4, '')
			WHERE id = $1`, id, newStatus, a.ID, notes); err != nil {
			return err
		}
		return Audit(ctx, tx, a, "duplicate."+action, "duplicate_pair", &id, nil, map[string]any{"keep": keep, "notes": notes})
	})
}

// MergeCompanies folds removeID into keepID inside tx. Related records move with their
// original source_id, so provenance and permissions survive; empty fields on the kept record are
// filled from the removed one together with their provenance. The removed record is archived,
// soft-deleted and its slug redirects to the kept record.
func MergeCompanies(ctx context.Context, tx DBTX, a Actor, keepID, removeID string) error {
	if keepID == removeID {
		return ErrInvalid{Msg: "cannot merge a company into itself"}
	}
	var keepDeleted, removeDeleted bool
	if err := tx.QueryRow(ctx, `SELECT
		(SELECT deleted_at IS NOT NULL FROM companies WHERE id = $1 FOR UPDATE),
		(SELECT deleted_at IS NOT NULL FROM companies WHERE id = $2 FOR UPDATE)`, keepID, removeID).Scan(&keepDeleted, &removeDeleted); err != nil {
		return err
	}
	if keepDeleted || removeDeleted {
		return ErrConflict{Msg: "one of the companies has already been deleted or merged"}
	}
	stmts := []string{
		`INSERT INTO company_roles (company_id, role_code, source_id) SELECT $1, role_code, source_id FROM company_roles WHERE company_id = $2 ON CONFLICT DO NOTHING`,
		`DELETE FROM company_roles WHERE company_id = $2`,
		`INSERT INTO company_categories (company_id, category_id, source_id, confidence, verified_at)
		 SELECT $1, category_id, source_id, confidence, verified_at FROM company_categories WHERE company_id = $2 ON CONFLICT DO NOTHING`,
		`DELETE FROM company_categories WHERE company_id = $2`,
		`INSERT INTO company_capabilities (company_id, capability_id, source_id, attributes, verification_status)
		 SELECT $1, capability_id, source_id, attributes, verification_status FROM company_capabilities WHERE company_id = $2 ON CONFLICT DO NOTHING`,
		`DELETE FROM company_capabilities WHERE company_id = $2`,
		`UPDATE facilities SET is_primary = false WHERE company_id = $2 AND EXISTS (SELECT 1 FROM facilities WHERE company_id = $1 AND is_primary AND deleted_at IS NULL)`,
		`UPDATE facilities SET company_id = $1 WHERE company_id = $2`,
		`UPDATE products SET company_id = $1 WHERE company_id = $2`,
		`UPDATE contacts SET company_id = $1 WHERE company_id = $2`,
		`UPDATE certifications SET company_id = $1 WHERE company_id = $2`,
		`UPDATE documents SET company_id = $1 WHERE company_id = $2`,
		`UPDATE verification_events SET company_id = $1 WHERE company_id = $2`,
		`UPDATE field_provenance SET company_id = $1 WHERE company_id = $2 AND entity_type <> 'company'`,
		`UPDATE slug_redirects SET company_id = $1 WHERE company_id = $2`,
		`UPDATE supplier_submissions SET company_id = $1 WHERE company_id = $2`,
	}
	for _, sql := range stmts {
		if !strings.Contains(sql, "$1") {
			sql += " AND $1::uuid IS NOT NULL" // every statement takes (keep, remove); give $1 a type
		}
		sql = strings.NewReplacer("$1", "$1::uuid", "$2", "$2::uuid").Replace(sql)
		sql = strings.ReplaceAll(sql, "::uuid::uuid", "::uuid")
		if _, err := tx.Exec(ctx, sql, keepID, removeID); err != nil {
			return fmt.Errorf("merge step failed: %w", err)
		}
	}
	// Fill empty company-level fields from the removed record and copy their provenance.
	for _, field := range []string{"legal_name", "description", "website_url", "establishment_year"} {
		tag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE companies k SET %[1]s = r.%[1]s FROM companies r
			WHERE k.id = $1 AND r.id = $2 AND k.%[1]s IS NULL AND r.%[1]s IS NOT NULL`, field), keepID, removeID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() > 0 {
			if field == "website_url" {
				if _, err := tx.Exec(ctx, `UPDATE companies k SET website_domain = r.website_domain FROM companies r WHERE k.id = $1 AND r.id = $2`, keepID, removeID); err != nil {
					return err
				}
			}
			if _, err := tx.Exec(ctx, `INSERT INTO field_provenance (company_id, entity_type, entity_id, field_name, source_id, observed_at, permission_status, confidence)
				SELECT $1, 'company', $1, field_name, source_id, observed_at, permission_status, confidence
				FROM field_provenance WHERE company_id = $2 AND entity_type = 'company' AND entity_id = $2 AND field_name = $3
				ON CONFLICT DO NOTHING`, keepID, removeID, field); err != nil {
				return err
			}
		}
	}
	// Most restrictive withdrawal wins.
	if _, err := tx.Exec(ctx, `UPDATE companies k SET
		public_listing_withdrawn_at = COALESCE(k.public_listing_withdrawn_at, r.public_listing_withdrawn_at),
		commercial_api_withdrawn_at = COALESCE(k.commercial_api_withdrawn_at, r.commercial_api_withdrawn_at),
		publication_status = CASE WHEN r.public_listing_withdrawn_at IS NOT NULL AND k.publication_status = 'published'
		                          THEN 'approved_private' ELSE k.publication_status END
		FROM companies r WHERE k.id = $1 AND r.id = $2`, keepID, removeID); err != nil {
		return err
	}
	var oldSlug string
	if err := tx.QueryRow(ctx, `UPDATE companies SET publication_status = 'archived', merged_into_id = $1, deleted_at = now()
		WHERE id = $2 RETURNING slug`, keepID, removeID).Scan(&oldSlug); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO slug_redirects (old_slug, company_id) VALUES ($1, $2) ON CONFLICT (old_slug) DO UPDATE SET company_id = $2`, oldSlug, keepID); err != nil {
		return err
	}
	// Other open pairs involving the removed company are no longer meaningful.
	if _, err := tx.Exec(ctx, `UPDATE duplicate_pairs SET status = 'dismissed', resolved_by = $2, resolved_at = now(),
		resolution_notes = 'company merged elsewhere' WHERE status = 'open' AND (company_a_id = $1 OR company_b_id = $1)
		AND NOT (company_a_id = LEAST($1, $3::uuid) AND company_b_id = GREATEST($1, $3::uuid))`, removeID, a.ID, keepID); err != nil {
		return err
	}
	return Audit(ctx, tx, a, "company.merge", "company", &keepID, map[string]any{"merged_company_id": removeID}, map[string]any{"kept_company_id": keepID})
}
