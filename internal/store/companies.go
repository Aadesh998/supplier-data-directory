package store

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/bharatmfg/directory/internal/normalize"
)

// CategoryRef is a compact category reference.
type CategoryRef struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// AdminCompanySummary is a row in the admin company list.
type AdminCompanySummary struct {
	ID                 string        `json:"id"`
	DisplayName        string        `json:"display_name"`
	LegalName          *string       `json:"legal_name"`
	Slug               string        `json:"slug"`
	PublicationStatus  string        `json:"publication_status"`
	VerificationStatus string        `json:"verification_status"`
	City               *string       `json:"city"`
	StateCode          *string       `json:"state_code"`
	WebsiteDomain      *string       `json:"website_domain"`
	Roles              []string      `json:"roles"`
	Categories         []CategoryRef `json:"categories"`
	CreatedAt          time.Time     `json:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at"`
}

// Facility mirrors facilities.
type Facility struct {
	ID                 string  `json:"id"`
	FacilityType       string  `json:"facility_type"`
	Name               *string `json:"name"`
	AddressLine1       *string `json:"address_line1"`
	AddressLine2       *string `json:"address_line2"`
	Locality           *string `json:"locality"`
	City               *string `json:"city"`
	District           *string `json:"district"`
	StateCode          *string `json:"state_code"`
	PostalCode         *string `json:"postal_code"`
	IsPrimary          bool    `json:"is_primary"`
	VerificationStatus string  `json:"verification_status"`
	SourceID           *string `json:"source_id"`
}

// Product mirrors products.
type Product struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description *string      `json:"description"`
	Unit        *string      `json:"unit"`
	Category    *CategoryRef `json:"category"`
	SourceID    *string      `json:"source_id"`
}

// Contact is an admin view of a contact; individual values are masked.
type Contact struct {
	ID          string  `json:"id"`
	ContactType string  `json:"contact_type"`
	Channel     string  `json:"channel"`
	Label       *string `json:"label"`
	Value       string  `json:"value"`
	Masked      bool    `json:"masked"`
	IsPublic    bool    `json:"is_public"`
	LawfulBasis *string `json:"lawful_basis_or_permission_ref"`
	SourceID    *string `json:"source_id"`
}

// CapabilityRef is a company capability.
type CapabilityRef struct {
	ID                 string `json:"id"`
	Slug               string `json:"slug"`
	Name               string `json:"name"`
	VerificationStatus string `json:"verification_status"`
}

// CertificationRef is a company certification.
type CertificationRef struct {
	ID                string  `json:"id"`
	CertificationType string  `json:"certification_type"`
	Issuer            *string `json:"issuer"`
	Status            string  `json:"status"`
}

// VerificationEvent is one verification check.
type VerificationEvent struct {
	ID           string     `json:"id"`
	FieldName    *string    `json:"field_name"`
	Method       string     `json:"method"`
	Result       string     `json:"result"`
	Notes        *string    `json:"notes"`
	CheckedAt    time.Time  `json:"checked_at"`
	CheckedBy    string     `json:"checked_by"`
	NextReviewAt *time.Time `json:"next_review_at"`
}

// PublishCheck explains whether a company may be published.
type PublishCheck struct {
	Allowed bool     `json:"allowed"`
	Reasons []string `json:"reasons"`
}

// AdminCompany is the full admin view of a company.
type AdminCompany struct {
	AdminCompanySummary
	NormalizedName        string              `json:"normalized_name"`
	Description           *string             `json:"description"`
	WebsiteURL            *string             `json:"website_url"`
	GSTIN                 *string             `json:"gstin"`
	EstablishmentYear     *int16              `json:"establishment_year"`
	BusinessStatus        string              `json:"business_status"`
	PrimaryCategoryID     *string             `json:"primary_category_id"`
	PublishedAt           *time.Time          `json:"published_at"`
	LastVerifiedAt        *time.Time          `json:"last_verified_at"`
	NextReviewAt          *time.Time          `json:"next_review_at"`
	PublicWithdrawnAt     *time.Time          `json:"public_listing_withdrawn_at"`
	CommercialWithdrawnAt *time.Time          `json:"commercial_api_withdrawn_at"`
	Facilities            []Facility          `json:"facilities"`
	Products              []Product           `json:"products"`
	Capabilities          []CapabilityRef     `json:"capabilities"`
	Certifications        []CertificationRef  `json:"certifications"`
	Contacts              []Contact           `json:"contacts"`
	Provenance            []Provenance        `json:"provenance"`
	VerificationEvents    []VerificationEvent `json:"verification_events"`
	PublishCheck          PublishCheck        `json:"publish_check"`
}

// CompanyFilter filters the admin list.
type CompanyFilter struct {
	Q                  string
	PublicationStatus  string
	VerificationStatus string
	Category           string // slug, includes descendants
	State              string // code
}

const summarySelect = `SELECT c.id, c.display_name, c.legal_name, c.slug, c.publication_status, c.verification_status,
	f.city, f.state_code, c.website_domain,
	COALESCE((SELECT array_agg(role_code ORDER BY role_code) FROM company_roles WHERE company_id = c.id), '{}'),
	COALESCE((SELECT json_agg(json_build_object('id', cat.id, 'slug', cat.slug, 'name', cat.name) ORDER BY cat.name)
	          FROM company_categories cc JOIN categories cat ON cat.id = cc.category_id WHERE cc.company_id = c.id), '[]'),
	c.created_at, c.updated_at
	FROM companies c
	LEFT JOIN LATERAL (SELECT city, state_code FROM facilities
	                   WHERE company_id = c.id AND deleted_at IS NULL ORDER BY is_primary DESC, created_at LIMIT 1) f ON true`

func scanSummary(row pgx.Row) (AdminCompanySummary, error) {
	var s AdminCompanySummary
	err := row.Scan(&s.ID, &s.DisplayName, &s.LegalName, &s.Slug, &s.PublicationStatus, &s.VerificationStatus,
		&s.City, &s.StateCode, &s.WebsiteDomain, &s.Roles, &s.Categories, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

// categorySubtreeSQL selects the ids of a category (by slug) and all its descendants; $N is the slug.
func categorySubtreeSQL(param string) string {
	return `(WITH RECURSIVE sub AS (SELECT id FROM categories WHERE slug = ` + param + `
		UNION ALL SELECT ch.id FROM categories ch JOIN sub ON ch.parent_id = sub.id) SELECT id FROM sub)`
}

// ListAdminCompanies returns a filtered page of companies (excluding deleted).
func ListAdminCompanies(ctx context.Context, q DBTX, f CompanyFilter, limit, offset int) ([]AdminCompanySummary, int, error) {
	where := []string{"c.deleted_at IS NULL"}
	args := []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.ReplaceAll(cond, "$?", "$"+strconv.Itoa(len(args))))
	}
	order := "c.updated_at DESC"
	if f.Q != "" {
		key := normalize.CompanyName(f.Q)
		add("(c.normalized_name % $? OR c.normalized_name LIKE '%' || $? || '%' OR c.website_domain = lower($?))", key)
		order = fmt.Sprintf("similarity(c.normalized_name, $%d) DESC, c.updated_at DESC", len(args))
	}
	if f.PublicationStatus != "" {
		add("c.publication_status = $?", f.PublicationStatus)
	}
	if f.VerificationStatus != "" {
		add("c.verification_status = $?", f.VerificationStatus)
	}
	if f.State != "" {
		add("EXISTS (SELECT 1 FROM facilities fx WHERE fx.company_id = c.id AND fx.deleted_at IS NULL AND fx.state_code = upper($?))", f.State)
	}
	if f.Category != "" {
		args = append(args, f.Category)
		where = append(where, "EXISTS (SELECT 1 FROM company_categories cx WHERE cx.company_id = c.id AND cx.category_id IN "+
			categorySubtreeSQL("$"+strconv.Itoa(len(args)))+")")
	}
	w := " WHERE " + strings.Join(where, " AND ")
	var total int
	if err := q.QueryRow(ctx, "SELECT count(*) FROM companies c"+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	rows, err := q.Query(ctx, summarySelect+w+" ORDER BY "+order+
		fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []AdminCompanySummary
	for rows.Next() {
		s, err := scanSummary(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, s)
	}
	return out, total, rows.Err()
}

// GetAdminCompany loads the complete admin view of a company.
func (s *Store) GetAdminCompany(ctx context.Context, q DBTX, id string) (AdminCompany, error) {
	var c AdminCompany
	sum, err := scanSummary(q.QueryRow(ctx, summarySelect+" WHERE c.id = $1", id))
	if err != nil {
		return c, err
	}
	c.AdminCompanySummary = sum
	err = q.QueryRow(ctx, `SELECT normalized_name, description, website_url, gstin, establishment_year, business_status,
		primary_category_id, published_at, last_verified_at, next_review_at, public_listing_withdrawn_at, commercial_api_withdrawn_at
		FROM companies WHERE id = $1`, id).Scan(&c.NormalizedName, &c.Description, &c.WebsiteURL, &c.GSTIN, &c.EstablishmentYear,
		&c.BusinessStatus, &c.PrimaryCategoryID, &c.PublishedAt, &c.LastVerifiedAt, &c.NextReviewAt,
		&c.PublicWithdrawnAt, &c.CommercialWithdrawnAt)
	if err != nil {
		return c, err
	}
	if c.Facilities, err = listFacilities(ctx, q, id); err != nil {
		return c, err
	}
	if c.Products, err = listProducts(ctx, q, id); err != nil {
		return c, err
	}
	if c.Capabilities, err = collect(ctx, q, `SELECT cap.id, cap.slug, cap.name, cc.verification_status
		FROM company_capabilities cc JOIN capabilities cap ON cap.id = cc.capability_id WHERE cc.company_id = $1 ORDER BY cap.name`,
		func(r pgx.Rows) (CapabilityRef, error) {
			var x CapabilityRef
			return x, r.Scan(&x.ID, &x.Slug, &x.Name, &x.VerificationStatus)
		}, id); err != nil {
		return c, err
	}
	if c.Certifications, err = collect(ctx, q, `SELECT id, certification_type, issuer, status FROM certifications
		WHERE company_id = $1 ORDER BY created_at`, func(r pgx.Rows) (CertificationRef, error) {
		var x CertificationRef
		return x, r.Scan(&x.ID, &x.CertificationType, &x.Issuer, &x.Status)
	}, id); err != nil {
		return c, err
	}
	if c.Contacts, err = s.listContacts(ctx, q, id); err != nil {
		return c, err
	}
	if c.Provenance, err = ListProvenance(ctx, q, id); err != nil {
		return c, err
	}
	if c.VerificationEvents, err = collect(ctx, q, `SELECT v.id, v.field_name, v.method, v.result, v.notes, v.checked_at,
		u.email::text, v.next_review_at FROM verification_events v JOIN admin_users u ON u.id = v.checked_by
		WHERE v.company_id = $1 ORDER BY v.checked_at DESC`, func(r pgx.Rows) (VerificationEvent, error) {
		var x VerificationEvent
		return x, r.Scan(&x.ID, &x.FieldName, &x.Method, &x.Result, &x.Notes, &x.CheckedAt, &x.CheckedBy, &x.NextReviewAt)
	}, id); err != nil {
		return c, err
	}
	reasons, err := CheckPublishable(ctx, q, id)
	if err != nil {
		return c, err
	}
	c.PublishCheck = PublishCheck{Allowed: len(reasons) == 0, Reasons: reasons}
	return c, nil
}

func collect[T any](ctx context.Context, q DBTX, sql string, scan func(pgx.Rows) (T, error), args ...any) ([]T, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func listFacilities(ctx context.Context, q DBTX, companyID string) ([]Facility, error) {
	return collect(ctx, q, `SELECT id, facility_type, name, address_line1, address_line2, locality, city, district,
		state_code, postal_code, is_primary, verification_status, source_id
		FROM facilities WHERE company_id = $1 AND deleted_at IS NULL ORDER BY is_primary DESC, created_at`,
		func(r pgx.Rows) (Facility, error) {
			var f Facility
			return f, r.Scan(&f.ID, &f.FacilityType, &f.Name, &f.AddressLine1, &f.AddressLine2, &f.Locality, &f.City,
				&f.District, &f.StateCode, &f.PostalCode, &f.IsPrimary, &f.VerificationStatus, &f.SourceID)
		}, companyID)
}

func listProducts(ctx context.Context, q DBTX, companyID string) ([]Product, error) {
	return collect(ctx, q, `SELECT p.id, p.name, p.description, p.unit, cat.id, cat.slug, cat.name, p.source_id
		FROM products p LEFT JOIN categories cat ON cat.id = p.category_id
		WHERE p.company_id = $1 AND p.deleted_at IS NULL ORDER BY p.name`,
		func(r pgx.Rows) (Product, error) {
			var p Product
			var cid, cslug, cname *string
			err := r.Scan(&p.ID, &p.Name, &p.Description, &p.Unit, &cid, &cslug, &cname, &p.SourceID)
			if cid != nil {
				p.Category = &CategoryRef{ID: *cid, Slug: deref(cslug), Name: deref(cname)}
			}
			return p, err
		}, companyID)
}

func (s *Store) listContacts(ctx context.Context, q DBTX, companyID string) ([]Contact, error) {
	type raw struct {
		Contact
		value *string
		enc   []byte
	}
	rows, err := collect(ctx, q, `SELECT id, contact_type, channel, label, value, value_encrypted, is_public,
		lawful_basis_or_permission_ref, source_id FROM contacts WHERE company_id = $1 AND deleted_at IS NULL ORDER BY created_at`,
		func(r pgx.Rows) (raw, error) {
			var x raw
			return x, r.Scan(&x.ID, &x.ContactType, &x.Channel, &x.Label, &x.value, &x.enc, &x.IsPublic, &x.LawfulBasis, &x.SourceID)
		}, companyID)
	if err != nil {
		return nil, err
	}
	out := make([]Contact, 0, len(rows))
	for _, r := range rows {
		c := r.Contact
		if r.value != nil {
			c.Value = *r.value
		} else {
			// Personal contacts are never returned in clear through list/detail endpoints.
			pt, err := s.Box.Decrypt(r.enc)
			if err != nil {
				c.Value = "[unreadable]"
			} else {
				c.Value = maskValue(c.Channel, pt)
			}
			c.Masked = true
		}
		out = append(out, c)
	}
	return out, nil
}

// CheckPublishable returns the reasons a company cannot be published (empty = publishable).
func CheckPublishable(ctx context.Context, q DBTX, id string) ([]string, error) {
	var status string
	var deleted, merged, withdrawn bool
	var namePermitted, hasPublicCategory, hasPublicLocation bool
	err := q.QueryRow(ctx, `SELECT c.publication_status, c.deleted_at IS NOT NULL, c.merged_into_id IS NOT NULL,
		c.public_listing_withdrawn_at IS NOT NULL,
		EXISTS (SELECT 1 FROM public_field_permissions p WHERE p.company_id = c.id AND p.entity_type = 'company'
		        AND p.entity_id = c.id AND p.field_name = 'display_name'),
		EXISTS (SELECT 1 FROM company_categories cc JOIN data_sources ds ON ds.id = cc.source_id
		        WHERE cc.company_id = c.id AND ds.allowed_public_display),
		EXISTS (SELECT 1 FROM facilities f JOIN public_field_permissions p ON p.entity_type = 'facility' AND p.entity_id = f.id
		        WHERE f.company_id = c.id AND f.deleted_at IS NULL AND p.field_name IN ('city', 'state_code'))
		FROM companies c WHERE c.id = $1`, id).Scan(&status, &deleted, &merged, &withdrawn, &namePermitted,
		&hasPublicCategory, &hasPublicLocation)
	if err != nil {
		return nil, err
	}
	reasons := []string{}
	if deleted || merged {
		reasons = append(reasons, "the company has been deleted or merged into another record")
	}
	if status != "approved_private" && status != "published" {
		reasons = append(reasons, fmt.Sprintf("the company must be approved (approved_private) before publishing; current status is %s", status))
	}
	if withdrawn {
		reasons = append(reasons, "the business has withdrawn permission for public listing")
	}
	if !namePermitted {
		reasons = append(reasons, "the display name is not backed by any source that permits public display")
	}
	if !hasPublicCategory {
		reasons = append(reasons, "at least one category backed by a public-display source is required")
	}
	if !hasPublicLocation {
		reasons = append(reasons, "at least one location (city or state) with public-display permission is required")
	}
	return reasons, nil
}

// CompanyInput is used for create (required: DisplayName, SourceID) and patch (all optional).
type CompanyInput struct {
	SourceID          *string   `json:"source_id" validate:"omitempty,uuid"`
	DisplayName       *string   `json:"display_name" validate:"omitempty,min=2,max=300"`
	LegalName         *string   `json:"legal_name" validate:"omitempty,max=300"`
	Description       *string   `json:"description" validate:"omitempty,max=5000"`
	WebsiteURL        *string   `json:"website_url" validate:"omitempty,max=500"`
	GSTIN             *string   `json:"gstin" validate:"omitempty,max=20"`
	EstablishmentYear *int      `json:"establishment_year" validate:"omitempty,gte=1800,lte=2100"`
	BusinessStatus    *string   `json:"business_status" validate:"omitempty,oneof=active inactive closed unknown"`
	PrimaryCategoryID *string   `json:"primary_category_id" validate:"omitempty,uuid"`
	Slug              *string   `json:"slug" validate:"omitempty,max=160"`
	Roles             *[]string `json:"roles" validate:"omitempty,max=15"`
	CategoryIDs       *[]string `json:"category_ids" validate:"omitempty,max=20,dive,uuid"`
	PublicationStatus *string   `json:"publication_status" validate:"omitempty,oneof=draft submitted under_review approved_private rejected needs_changes archived"`
}

func (in CompanyInput) dataFields() []string {
	var f []string
	if in.DisplayName != nil {
		f = append(f, "display_name")
	}
	if in.LegalName != nil {
		f = append(f, "legal_name")
	}
	if in.Description != nil {
		f = append(f, "description")
	}
	if in.WebsiteURL != nil {
		f = append(f, "website_url")
	}
	if in.GSTIN != nil {
		f = append(f, "gstin")
	}
	if in.EstablishmentYear != nil {
		f = append(f, "establishment_year")
	}
	return f
}

// cleanGSTIN validates an optional GSTIN; an empty string clears it.
func cleanGSTIN(v *string) (*string, error) {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil, nil
	}
	g, ok := normalize.GSTIN(*v)
	if !ok {
		return nil, ErrInvalid{Msg: "invalid GSTIN", Fields: map[string]string{"gstin": "must be a valid 15-character GSTIN"}}
	}
	return &g, nil
}

// gstinConflict turns a unique-GSTIN violation into a readable conflict.
func gstinConflict(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == "companies_gstin_key" {
		return ErrConflict{Msg: "another company already has this GSTIN; merge the records instead"}
	}
	return err
}

// uniqueSlug returns base, or base-2, base-3... that is not used by a company or redirect.
func uniqueSlug(ctx context.Context, q DBTX, base string) (string, error) {
	if base == "" {
		base = "company"
	}
	candidate := base
	for i := 2; i < 1000; i++ {
		var taken bool
		err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM companies WHERE slug = $1)
			OR EXISTS (SELECT 1 FROM slug_redirects WHERE old_slug = $1)`, candidate).Scan(&taken)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
	return "", fmt.Errorf("could not allocate slug for %q", base)
}

func cleanWebsite(in *string) (url, domain *string, err error) {
	if in == nil || strings.TrimSpace(*in) == "" {
		return nil, nil, nil
	}
	u, d, ok := normalize.WebsiteURL(*in)
	if !ok {
		return nil, nil, ErrInvalid{Msg: "invalid website", Fields: map[string]string{"website_url": "must be a public http(s) website address"}}
	}
	return &u, &d, nil
}

// requireInternalSource ensures sourceID exists and allows internal use.
func requireInternalSource(ctx context.Context, q DBTX, sourceID string) error {
	var ok bool
	err := q.QueryRow(ctx, `SELECT allowed_internal_use FROM data_sources WHERE id = $1`, sourceID).Scan(&ok)
	if IsNotFound(err) {
		return ErrInvalid{Msg: "unknown source", Fields: map[string]string{"source_id": "does not exist"}}
	}
	if err != nil {
		return err
	}
	if !ok {
		return ErrPolicy{Code: "source_not_permitted", Msg: "the selected source does not permit internal use; data from it must stay in staging"}
	}
	return nil
}

// CreateCompany creates a company as draft with provenance for each supplied field.
func (s *Store) CreateCompany(ctx context.Context, a Actor, in CompanyInput) (AdminCompany, error) {
	if in.DisplayName == nil || in.SourceID == nil {
		return AdminCompany{}, ErrInvalid{Msg: "display_name and source_id are required", Fields: map[string]string{
			"display_name": "is required", "source_id": "is required so the data has provenance"}}
	}
	var id string
	err := s.InTx(ctx, func(tx pgx.Tx) error {
		var err error
		id, err = createCompanyTx(ctx, tx, a, in, "draft")
		return err
	})
	if err != nil {
		return AdminCompany{}, err
	}
	return s.GetAdminCompany(ctx, s.Pool, id)
}

func createCompanyTx(ctx context.Context, tx DBTX, a Actor, in CompanyInput, status string) (string, error) {
	if err := requireInternalSource(ctx, tx, *in.SourceID); err != nil {
		return "", err
	}
	name := normalize.Whitespace(*in.DisplayName)
	url, domain, err := cleanWebsite(in.WebsiteURL)
	if err != nil {
		return "", err
	}
	slug, err := uniqueSlug(ctx, tx, normalize.Slug(name))
	if err != nil {
		return "", err
	}
	var legal, desc *string
	if in.LegalName != nil {
		legal = strOrNil(normalize.Whitespace(*in.LegalName))
	}
	if in.Description != nil {
		desc = strOrNil(strings.TrimSpace(*in.Description))
	}
	bs := "unknown"
	if in.BusinessStatus != nil {
		bs = *in.BusinessStatus
	}
	gstin, err := cleanGSTIN(in.GSTIN)
	if err != nil {
		return "", err
	}
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO companies (display_name, legal_name, slug, normalized_name, description,
		website_url, website_domain, establishment_year, business_status, publication_status, primary_category_id, gstin)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id`,
		name, legal, slug, normalize.CompanyName(name), desc, url, domain, in.EstablishmentYear, bs, status,
		in.PrimaryCategoryID, gstin).Scan(&id)
	if err != nil {
		return "", gstinConflict(err)
	}
	fields := []string{"display_name"}
	if gstin != nil {
		fields = append(fields, "gstin")
	}
	if legal != nil {
		fields = append(fields, "legal_name")
	}
	if desc != nil {
		fields = append(fields, "description")
	}
	if url != nil {
		fields = append(fields, "website_url")
	}
	if in.EstablishmentYear != nil {
		fields = append(fields, "establishment_year")
	}
	if err := RecordProvenance(ctx, tx, id, "company", id, *in.SourceID, fields...); err != nil {
		return "", err
	}
	if in.Roles != nil {
		if err := setRoles(ctx, tx, id, *in.Roles, *in.SourceID); err != nil {
			return "", err
		}
	}
	if in.CategoryIDs != nil {
		if err := setCategories(ctx, tx, id, *in.CategoryIDs, *in.SourceID); err != nil {
			return "", err
		}
	}
	return id, Audit(ctx, tx, a, "company.create", "company", &id, nil, in)
}

func setRoles(ctx context.Context, tx DBTX, companyID string, roles []string, sourceID string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM company_roles WHERE company_id = $1 AND NOT (role_code = ANY($2))`, companyID, roles); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO company_roles (company_id, role_code, source_id)
		SELECT $1, unnest($2::text[]), $3 ON CONFLICT DO NOTHING`, companyID, roles, strOrNil(sourceID))
	return err
}

func setCategories(ctx context.Context, tx DBTX, companyID string, ids []string, sourceID string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM company_categories WHERE company_id = $1 AND NOT (category_id = ANY($2::uuid[]))`, companyID, ids); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO company_categories (company_id, category_id, source_id)
		SELECT $1, unnest($2::uuid[]), $3 ON CONFLICT DO NOTHING`, companyID, ids, strOrNil(sourceID))
	return err
}

// UpdateCompany applies a patch. Data fields require a source_id; the new values get provenance.
func (s *Store) UpdateCompany(ctx context.Context, a Actor, id string, in CompanyInput) (AdminCompany, error) {
	fields := in.dataFields()
	needsSource := len(fields) > 0 || in.Roles != nil || in.CategoryIDs != nil
	if needsSource && in.SourceID == nil {
		return AdminCompany{}, ErrInvalid{Msg: "source_id is required when changing data",
			Fields: map[string]string{"source_id": "is required so the changed data has provenance"}}
	}
	err := s.InTx(ctx, func(tx pgx.Tx) error {
		before, err := s.GetAdminCompany(ctx, tx, id)
		if err != nil {
			return err
		}
		if needsSource {
			if err := requireInternalSource(ctx, tx, *in.SourceID); err != nil {
				return err
			}
		}
		sets := []string{}
		args := []any{id}
		set := func(col string, v any) {
			args = append(args, v)
			sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
		}
		if in.DisplayName != nil {
			n := normalize.Whitespace(*in.DisplayName)
			set("display_name", n)
			set("normalized_name", normalize.CompanyName(n))
		}
		if in.LegalName != nil {
			set("legal_name", strOrNil(normalize.Whitespace(*in.LegalName)))
		}
		if in.Description != nil {
			set("description", strOrNil(strings.TrimSpace(*in.Description)))
		}
		if in.WebsiteURL != nil {
			u, d, err := cleanWebsite(in.WebsiteURL)
			if err != nil {
				return err
			}
			set("website_url", u)
			set("website_domain", d)
		}
		if in.GSTIN != nil {
			g, err := cleanGSTIN(in.GSTIN)
			if err != nil {
				return err
			}
			set("gstin", g)
		}
		if in.EstablishmentYear != nil {
			set("establishment_year", *in.EstablishmentYear)
		}
		if in.BusinessStatus != nil {
			set("business_status", *in.BusinessStatus)
		}
		if in.PrimaryCategoryID != nil {
			set("primary_category_id", *in.PrimaryCategoryID)
		}
		if in.PublicationStatus != nil {
			set("publication_status", *in.PublicationStatus)
		}
		if in.Slug != nil && *in.Slug != before.Slug {
			newSlug := normalize.Slug(*in.Slug)
			if newSlug == "" || newSlug != *in.Slug {
				return ErrInvalid{Msg: "invalid slug", Fields: map[string]string{"slug": "use lowercase letters, digits and single hyphens"}}
			}
			if _, err := tx.Exec(ctx, `DELETE FROM slug_redirects WHERE old_slug = $1 AND company_id = $2`, newSlug, id); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO slug_redirects (old_slug, company_id) VALUES ($1, $2)`, before.Slug, id); err != nil {
				return err
			}
			set("slug", newSlug)
		}
		if len(sets) > 0 {
			if _, err := tx.Exec(ctx, "UPDATE companies SET "+strings.Join(sets, ", ")+" WHERE id = $1 AND deleted_at IS NULL", args...); err != nil {
				return gstinConflict(err)
			}
		}
		if len(fields) > 0 {
			if err := RecordProvenance(ctx, tx, id, "company", id, *in.SourceID, fields...); err != nil {
				return err
			}
		}
		if in.Roles != nil {
			if err := setRoles(ctx, tx, id, *in.Roles, *in.SourceID); err != nil {
				return err
			}
		}
		if in.CategoryIDs != nil {
			if err := setCategories(ctx, tx, id, *in.CategoryIDs, *in.SourceID); err != nil {
				return err
			}
		}
		return Audit(ctx, tx, a, "company.update", "company", &id, before.AdminCompanySummary, in)
	})
	if err != nil {
		return AdminCompany{}, err
	}
	return s.GetAdminCompany(ctx, s.Pool, id)
}

// FacilityInput adds a facility.
type FacilityInput struct {
	SourceID     string  `json:"source_id" validate:"required,uuid"`
	FacilityType string  `json:"facility_type" validate:"required,oneof=factory office registered_office warehouse showroom workshop laboratory other"`
	Name         *string `json:"name" validate:"omitempty,max=200"`
	AddressLine1 *string `json:"address_line1" validate:"omitempty,max=200"`
	AddressLine2 *string `json:"address_line2" validate:"omitempty,max=200"`
	Locality     *string `json:"locality" validate:"omitempty,max=120"`
	City         *string `json:"city" validate:"omitempty,max=120"`
	District     *string `json:"district" validate:"omitempty,max=120"`
	StateCode    *string `json:"state_code" validate:"omitempty,max=60"`
	PostalCode   *string `json:"postal_code" validate:"omitempty,max=10"`
	IsPrimary    bool    `json:"is_primary"`
}

func insertFacility(ctx context.Context, tx DBTX, companyID string, in FacilityInput) (string, error) {
	var state *string
	if in.StateCode != nil && *in.StateCode != "" {
		code, ok := normalize.StateCode(*in.StateCode)
		if !ok {
			return "", ErrInvalid{Msg: "unknown state", Fields: map[string]string{"state_code": "is not a recognised Indian state or union territory"}}
		}
		state = &code
	}
	var pin *string
	if in.PostalCode != nil && *in.PostalCode != "" {
		p, ok := normalize.PostalCode(*in.PostalCode)
		if !ok {
			return "", ErrInvalid{Msg: "invalid PIN code", Fields: map[string]string{"postal_code": "must be a 6-digit Indian PIN code"}}
		}
		pin = &p
	}
	clean := func(p *string) *string {
		if p == nil {
			return nil
		}
		return strOrNil(normalize.Whitespace(*p))
	}
	title := func(p *string) *string {
		if p == nil {
			return nil
		}
		return strOrNil(normalize.Title(*p))
	}
	if in.IsPrimary {
		if _, err := tx.Exec(ctx, `UPDATE facilities SET is_primary = false WHERE company_id = $1 AND is_primary`, companyID); err != nil {
			return "", err
		}
	} else {
		var hasPrimary bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM facilities WHERE company_id = $1 AND is_primary AND deleted_at IS NULL)`, companyID).Scan(&hasPrimary); err != nil {
			return "", err
		}
		in.IsPrimary = !hasPrimary
	}
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO facilities (company_id, facility_type, name, address_line1, address_line2, locality,
		city, district, state_code, postal_code, is_primary, source_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id`,
		companyID, in.FacilityType, clean(in.Name), clean(in.AddressLine1), clean(in.AddressLine2), title(in.Locality),
		title(in.City), title(in.District), state, pin, in.IsPrimary, in.SourceID).Scan(&id)
	if err != nil {
		return "", err
	}
	fields := []string{"facility_type"}
	for name, v := range map[string]*string{"name": in.Name, "address_line1": in.AddressLine1, "address_line2": in.AddressLine2,
		"locality": in.Locality, "city": in.City, "district": in.District, "state_code": state, "postal_code": pin} {
		if v != nil && *v != "" {
			fields = append(fields, name)
		}
	}
	return id, RecordProvenance(ctx, tx, companyID, "facility", id, in.SourceID, fields...)
}

// AddFacility adds a facility to a company.
func (s *Store) AddFacility(ctx context.Context, a Actor, companyID string, in FacilityInput) error {
	return s.InTx(ctx, func(tx pgx.Tx) error {
		if err := requireInternalSource(ctx, tx, in.SourceID); err != nil {
			return err
		}
		fid, err := insertFacility(ctx, tx, companyID, in)
		if err != nil {
			return err
		}
		return Audit(ctx, tx, a, "facility.create", "company", &companyID, nil, map[string]any{"facility_id": fid, "input": in})
	})
}

// RemoveFacility soft-deletes a facility.
func (s *Store) RemoveFacility(ctx context.Context, a Actor, companyID, facilityID string) error {
	return s.InTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE facilities SET deleted_at = now(), is_primary = false
			WHERE id = $1 AND company_id = $2 AND deleted_at IS NULL`, facilityID, companyID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return Audit(ctx, tx, a, "facility.delete", "company", &companyID, map[string]any{"facility_id": facilityID}, nil)
	})
}

// ProductInput adds a product.
type ProductInput struct {
	SourceID    string  `json:"source_id" validate:"required,uuid"`
	Name        string  `json:"name" validate:"required,max=300"`
	Description *string `json:"description" validate:"omitempty,max=3000"`
	Unit        *string `json:"unit" validate:"omitempty,max=50"`
	CategoryID  *string `json:"category_id" validate:"omitempty,uuid"`
}

func insertProduct(ctx context.Context, tx DBTX, companyID string, in ProductInput) (string, error) {
	name := normalize.Whitespace(in.Name)
	if name == "" {
		return "", ErrInvalid{Msg: "product name required", Fields: map[string]string{"name": "is required"}}
	}
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO products (company_id, category_id, name, normalized_name, description, unit, source_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`, companyID, in.CategoryID, name, strings.ToLower(name),
		in.Description, in.Unit, in.SourceID).Scan(&id)
	if err != nil {
		return "", err
	}
	fields := []string{"name"}
	if in.Description != nil && *in.Description != "" {
		fields = append(fields, "description")
	}
	if in.Unit != nil && *in.Unit != "" {
		fields = append(fields, "unit")
	}
	return id, RecordProvenance(ctx, tx, companyID, "product", id, in.SourceID, fields...)
}

// AddProduct adds a product to a company.
func (s *Store) AddProduct(ctx context.Context, a Actor, companyID string, in ProductInput) error {
	return s.InTx(ctx, func(tx pgx.Tx) error {
		if err := requireInternalSource(ctx, tx, in.SourceID); err != nil {
			return err
		}
		pid, err := insertProduct(ctx, tx, companyID, in)
		if err != nil {
			return err
		}
		return Audit(ctx, tx, a, "product.create", "company", &companyID, nil, map[string]any{"product_id": pid, "input": in})
	})
}

// RemoveProduct soft-deletes a product.
func (s *Store) RemoveProduct(ctx context.Context, a Actor, companyID, productID string) error {
	return s.InTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE products SET deleted_at = now() WHERE id = $1 AND company_id = $2 AND deleted_at IS NULL`, productID, companyID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return Audit(ctx, tx, a, "product.delete", "company", &companyID, map[string]any{"product_id": productID}, nil)
	})
}

// Publish makes a company public after policy checks. Publishing never changes verification status.
func (s *Store) Publish(ctx context.Context, a Actor, id string) error {
	return s.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT 1 FROM companies WHERE id = $1 FOR UPDATE`, id); err != nil {
			return err
		}
		reasons, err := CheckPublishable(ctx, tx, id)
		if err != nil {
			return err
		}
		if len(reasons) > 0 {
			return ErrPolicy{Code: "publish_blocked", Msg: "this company cannot be published yet", Reasons: reasons}
		}
		if _, err := tx.Exec(ctx, `UPDATE companies SET publication_status = 'published', published_at = COALESCE(published_at, now())
			WHERE id = $1`, id); err != nil {
			return err
		}
		return Audit(ctx, tx, a, "company.publish", "company", &id, nil, map[string]any{"publication_status": "published"})
	})
}

// Unpublish returns a published company to approved_private.
func (s *Store) Unpublish(ctx context.Context, a Actor, id, reason string) error {
	return s.InTx(ctx, func(tx pgx.Tx) error {
		return unpublishTx(ctx, tx, a, id, reason)
	})
}

func unpublishTx(ctx context.Context, tx DBTX, a Actor, id, reason string) error {
	tag, err := tx.Exec(ctx, `UPDATE companies SET publication_status = 'approved_private'
		WHERE id = $1 AND publication_status = 'published'`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM companies WHERE id = $1)`, id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return pgx.ErrNoRows
		}
		return nil // already not published: idempotent
	}
	return Audit(ctx, tx, a, "company.unpublish", "company", &id, map[string]any{"publication_status": "published"},
		map[string]any{"publication_status": "approved_private", "reason": reason})
}

// Archive archives a company (unpublishing it first).
func (s *Store) Archive(ctx context.Context, a Actor, id string) error {
	return s.InTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE companies SET publication_status = 'archived' WHERE id = $1 AND deleted_at IS NULL`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return Audit(ctx, tx, a, "company.archive", "company", &id, nil, map[string]any{"publication_status": "archived"})
	})
}

// UnpublishNonCompliant unpublishes every published company that no longer passes the publish policy.
func UnpublishNonCompliant(ctx context.Context, tx DBTX, a Actor, reason string) ([]string, error) {
	ids, err := collect(ctx, tx, `SELECT id FROM companies WHERE publication_status = 'published'`, func(r pgx.Rows) (string, error) {
		var id string
		return id, r.Scan(&id)
	})
	if err != nil {
		return nil, err
	}
	var out []string
	for _, id := range ids {
		reasons, err := CheckPublishable(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		if len(reasons) > 0 {
			if err := unpublishTx(ctx, tx, a, id, reason+": "+strings.Join(reasons, "; ")); err != nil {
				return nil, err
			}
			out = append(out, id)
		}
	}
	return out, nil
}

// VerifyInput records a verification check.
type VerifyInput struct {
	Method         string  `json:"method" validate:"required,oneof=supplier_confirmation official_document website_check manual_review phone_call"`
	Result         string  `json:"result" validate:"required,oneof=confirmed partially_confirmed not_confirmed inconclusive"`
	FieldName      *string `json:"field_name" validate:"omitempty,max=60"`
	Notes          *string `json:"notes" validate:"omitempty,max=4000"`
	NextReviewDays *int    `json:"next_review_days" validate:"omitempty,gte=1,lte=1095"`
}

// Verify records a verification event and recomputes the company's verification status.
// Verification is independent of publication.
func (s *Store) Verify(ctx context.Context, a Actor, id string, in VerifyInput, defaultInterval time.Duration) error {
	interval := defaultInterval
	if in.NextReviewDays != nil {
		interval = time.Duration(*in.NextReviewDays) * 24 * time.Hour
	}
	next := time.Now().Add(interval)
	return s.InTx(ctx, func(tx pgx.Tx) error {
		var current string
		if err := tx.QueryRow(ctx, `SELECT verification_status FROM companies WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&current); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO verification_events (company_id, field_name, method, result, notes, checked_by, next_review_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`, id, in.FieldName, in.Method, in.Result, in.Notes, a.ID, next); err != nil {
			return err
		}
		newStatus := current
		fieldLevel := in.FieldName != nil && *in.FieldName != ""
		switch in.Result {
		case "confirmed":
			if fieldLevel {
				if current != "verified" {
					newStatus = "partially_verified"
				}
			} else {
				newStatus = "verified"
			}
		case "partially_confirmed":
			newStatus = "partially_verified"
		case "not_confirmed":
			newStatus = "unverified"
		}
		touch := in.Result == "confirmed" || in.Result == "partially_confirmed"
		if _, err := tx.Exec(ctx, `UPDATE companies SET verification_status = $2,
			last_verified_at = CASE WHEN $3 THEN now() ELSE last_verified_at END,
			next_review_at = CASE WHEN $3 THEN $4 ELSE next_review_at END WHERE id = $1`, id, newStatus, touch, next); err != nil {
			return err
		}
		return Audit(ctx, tx, a, "company.verify", "company", &id, map[string]any{"verification_status": current},
			map[string]any{"verification_status": newStatus, "method": in.Method, "result": in.Result, "field_name": in.FieldName})
	})
}

// MarkStale flags verified companies whose review date has passed.
func MarkStale(ctx context.Context, q DBTX) (int64, error) {
	tag, err := q.Exec(ctx, `UPDATE companies SET verification_status = 'stale'
		WHERE verification_status IN ('verified', 'partially_verified') AND next_review_at < now() AND deleted_at IS NULL`)
	return tag.RowsAffected(), err
}
