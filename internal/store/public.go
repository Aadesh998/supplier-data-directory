package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bharatmfg/directory/internal/normalize"
)

// PublicCompanySummary contains only publicly permitted fields; empty fields are omitted.
type PublicCompanySummary struct {
	Slug               string        `json:"slug"`
	DisplayName        string        `json:"display_name"`
	Description        string        `json:"description,omitempty"`
	City               string        `json:"city,omitempty"`
	StateCode          string        `json:"state_code,omitempty"`
	StateName          string        `json:"state_name,omitempty"`
	Roles              []string      `json:"roles"`
	Categories         []CategoryRef `json:"categories"`
	VerificationStatus string        `json:"verification_status"`
	UpdatedAt          time.Time     `json:"updated_at"`
}

// PublicFacility is a public facility view.
type PublicFacility struct {
	FacilityType string `json:"facility_type,omitempty"`
	Locality     string `json:"locality,omitempty"`
	City         string `json:"city,omitempty"`
	District     string `json:"district,omitempty"`
	StateCode    string `json:"state_code,omitempty"`
	StateName    string `json:"state_name,omitempty"`
}

// PublicProduct is a public product view.
type PublicProduct struct {
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Category    *CategoryRef `json:"category,omitempty"`
}

// PublicContact is a public business contact.
type PublicContact struct {
	ContactType string `json:"contact_type"`
	Channel     string `json:"channel"`
	Label       string `json:"label,omitempty"`
	Value       string `json:"value"`
}

// PublicCapability is a public capability claim.
type PublicCapability struct {
	Slug               string `json:"slug"`
	Name               string `json:"name"`
	VerificationStatus string `json:"verification_status"`
}

// PublicCertification is a public certification claim with its honest status.
type PublicCertification struct {
	CertificationType string `json:"certification_type"`
	Issuer            string `json:"issuer,omitempty"`
	Status            string `json:"status"`
}

// PublicCompany is the full public profile.
type PublicCompany struct {
	PublicCompanySummary
	LegalName         string                `json:"legal_name,omitempty"`
	WebsiteURL        string                `json:"website_url,omitempty"`
	EstablishmentYear int                   `json:"establishment_year,omitempty"`
	Facilities        []PublicFacility      `json:"facilities"`
	Products          []PublicProduct       `json:"products"`
	Capabilities      []PublicCapability    `json:"capabilities"`
	Certifications    []PublicCertification `json:"certifications"`
	Contacts          []PublicContact       `json:"contacts"`
	SourceLabels      []string              `json:"source_labels"`
	PublishedAt       *time.Time            `json:"published_at,omitempty"`
	LastVerifiedAt    *time.Time            `json:"last_verified_at,omitempty"`
}

// PublicFilter filters public search.
type PublicFilter struct {
	Q        string
	Category string
	State    string
	City     string
	Role     string
}

// publishedWhere is the single definition of "publicly visible company".
// The permission probe matches public_field_permissions but queries the base tables by entity_id
// so the planner uses field_provenance_entity_idx (filtering on company_id as well causes a
// badly misestimated plan at scale).
const publishedWhere = `c.publication_status = 'published' AND c.deleted_at IS NULL AND c.public_listing_withdrawn_at IS NULL
	AND EXISTS (SELECT 1 FROM field_provenance pp JOIN data_sources pds ON pds.id = pp.source_id
	            WHERE pp.entity_id = c.id AND pp.entity_type = 'company' AND pp.field_name = 'display_name'
	              AND pp.permission_status = 'permitted' AND pds.allowed_public_display
	              AND (pds.retention_expires_at IS NULL OR pds.retention_expires_at > now()))`

// publicSource is a correlated predicate: the row's source permits public display (or commercial use).
func sourceAllows(purpose Purpose, col string) string {
	return fmt.Sprintf(`EXISTS (SELECT 1 FROM data_sources dsx WHERE dsx.id = %s AND dsx.%s
		AND (dsx.retention_expires_at IS NULL OR dsx.retention_expires_at > now()))`, col, purpose.sourceFlag())
}

type rawCompany struct {
	id, slug, displayName, verification string
	legalName, description, website     *string
	year                                *int16
	updatedAt                           time.Time
	publishedAt, lastVerifiedAt         *time.Time
}

// MaxPublicTotal caps the reported total for public searches.
const MaxPublicTotal = 10000

// SearchPublicCompanies returns a page of published companies with only permitted fields.
func SearchPublicCompanies(ctx context.Context, q DBTX, f PublicFilter, limit, offset int) ([]PublicCompanySummary, int, error) {
	where := []string{publishedWhere}
	args := []any{}
	next := func(v any) string { args = append(args, v); return "$" + strconv.Itoa(len(args)) }
	order := "c.display_name"
	if f.Q != "" {
		key := normalize.CompanyName(f.Q)
		if key != "" {
			p := next(key)
			// UNION of index-backed candidate sets (an OR across these predicates defeats the trigram index).
			where = append(where, fmt.Sprintf(`c.id IN (
				SELECT id FROM companies WHERE normalized_name %% %[1]s AND deleted_at IS NULL
				UNION SELECT id FROM companies WHERE normalized_name LIKE '%%' || %[1]s || '%%' AND deleted_at IS NULL
				UNION SELECT px.company_id FROM products px WHERE px.deleted_at IS NULL AND px.normalized_name LIKE '%%' || %[1]s || '%%'
				  AND EXISTS (SELECT 1 FROM public_field_permissions pq WHERE pq.entity_type = 'product'
				              AND pq.entity_id = px.id AND pq.field_name = 'name'))`, p))
			order = fmt.Sprintf("similarity(c.normalized_name, %s) DESC, c.display_name", p)
		}
	}
	if f.Category != "" {
		where = append(where, `EXISTS (SELECT 1 FROM company_categories cx WHERE cx.company_id = c.id
			AND cx.category_id IN `+categorySubtreeSQL(next(f.Category))+` AND `+sourceAllows(PurposePublic, "cx.source_id")+`)`)
	}
	if f.State != "" {
		state := f.State
		if code, ok := normalize.StateCode(strings.ReplaceAll(state, "-", " ")); ok {
			state = code
		}
		where = append(where, `EXISTS (SELECT 1 FROM facilities fx JOIN public_field_permissions pf ON pf.entity_type = 'facility'
			AND pf.entity_id = fx.id AND pf.field_name = 'state_code'
			WHERE fx.company_id = c.id AND fx.deleted_at IS NULL AND fx.state_code = upper(`+next(state)+`))`)
	}
	if f.City != "" {
		where = append(where, `EXISTS (SELECT 1 FROM facilities fx JOIN public_field_permissions pf ON pf.entity_type = 'facility'
			AND pf.entity_id = fx.id AND pf.field_name = 'city'
			WHERE fx.company_id = c.id AND fx.deleted_at IS NULL AND lower(fx.city) = lower(`+next(normalize.Whitespace(f.City))+`))`)
	}
	if f.Role != "" {
		where = append(where, `EXISTS (SELECT 1 FROM company_roles rx WHERE rx.company_id = c.id AND rx.role_code = `+
			next(f.Role)+` AND `+sourceAllows(PurposePublic, "rx.source_id")+`)`)
	}
	w := " WHERE " + strings.Join(where, " AND ")
	// Totals are capped: exact counts over very large result sets are costly and not useful to visitors.
	var total int
	if err := q.QueryRow(ctx, fmt.Sprintf("SELECT count(*) FROM (SELECT 1 FROM companies c%s LIMIT %d) t", w, MaxPublicTotal), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	sql := `SELECT c.id, c.slug, c.display_name, c.verification_status, c.legal_name, c.description, c.website_url,
		c.establishment_year, c.updated_at, c.published_at, c.last_verified_at FROM companies c` + w +
		" ORDER BY " + order + " LIMIT " + next(limit) + " OFFSET " + next(offset)
	raws, err := collect(ctx, q, sql, scanRaw, args...)
	if err != nil {
		return nil, 0, err
	}
	out, err := buildSummaries(ctx, q, PurposePublic, raws)
	return out, total, err
}

func scanRaw(r pgx.Rows) (rawCompany, error) {
	var c rawCompany
	return c, r.Scan(&c.id, &c.slug, &c.displayName, &c.verification, &c.legalName, &c.description, &c.website,
		&c.year, &c.updatedAt, &c.publishedAt, &c.lastVerifiedAt)
}

func buildSummaries(ctx context.Context, q DBTX, purpose Purpose, raws []rawCompany) ([]PublicCompanySummary, error) {
	ids := make([]string, len(raws))
	for i, r := range raws {
		ids[i] = r.id
	}
	perms, err := loadPermissions(ctx, q, purpose, ids)
	if err != nil {
		return nil, err
	}
	roles, err := permittedRoles(ctx, q, purpose, ids)
	if err != nil {
		return nil, err
	}
	cats, err := permittedCategories(ctx, q, purpose, ids)
	if err != nil {
		return nil, err
	}
	locs, err := primaryLocations(ctx, q, ids, perms)
	if err != nil {
		return nil, err
	}
	out := make([]PublicCompanySummary, 0, len(raws))
	for _, r := range raws {
		s := PublicCompanySummary{Slug: r.slug, DisplayName: r.displayName, VerificationStatus: r.verification,
			UpdatedAt: r.updatedAt, Roles: roles[r.id], Categories: cats[r.id]}
		if s.Roles == nil {
			s.Roles = []string{}
		}
		if s.Categories == nil {
			s.Categories = []CategoryRef{}
		}
		if r.description != nil && perms.has(r.id, "company", r.id, "description") {
			s.Description = *r.description
		}
		if l, ok := locs[r.id]; ok {
			s.City, s.StateCode, s.StateName = l.City, l.StateCode, l.StateName
		}
		out = append(out, s)
	}
	return out, nil
}

func permittedRoles(ctx context.Context, q DBTX, purpose Purpose, ids []string) (map[string][]string, error) {
	out := map[string][]string{}
	rows, err := collect(ctx, q, `SELECT cr.company_id, cr.role_code FROM company_roles cr
		WHERE cr.company_id = ANY($1::uuid[]) AND `+sourceAllows(purpose, "cr.source_id")+` ORDER BY cr.role_code`,
		func(r pgx.Rows) ([2]string, error) {
			var x [2]string
			return x, r.Scan(&x[0], &x[1])
		}, ids)
	for _, r := range rows {
		out[r[0]] = append(out[r[0]], r[1])
	}
	return out, err
}

func permittedCategories(ctx context.Context, q DBTX, purpose Purpose, ids []string) (map[string][]CategoryRef, error) {
	out := map[string][]CategoryRef{}
	type row struct {
		company string
		ref     CategoryRef
	}
	rows, err := collect(ctx, q, `SELECT cc.company_id, cat.id, cat.slug, cat.name FROM company_categories cc
		JOIN categories cat ON cat.id = cc.category_id AND cat.active
		WHERE cc.company_id = ANY($1::uuid[]) AND `+sourceAllows(purpose, "cc.source_id")+` ORDER BY cat.name`,
		func(r pgx.Rows) (row, error) {
			var x row
			return x, r.Scan(&x.company, &x.ref.ID, &x.ref.Slug, &x.ref.Name)
		}, ids)
	for _, r := range rows {
		out[r.company] = append(out[r.company], r.ref)
	}
	return out, err
}

type facilityRow struct {
	companyID, id                                string
	facilityType                                 string
	locality, city, district, stateCode, stateNm *string
}

func facilitiesFor(ctx context.Context, q DBTX, ids []string, primaryOnly bool) ([]facilityRow, error) {
	sql := `SELECT f.company_id, f.id, f.facility_type, f.locality, f.city, f.district, f.state_code, s.name
		FROM facilities f LEFT JOIN indian_states s ON s.code = f.state_code
		WHERE f.company_id = ANY($1::uuid[]) AND f.deleted_at IS NULL ORDER BY f.company_id, f.is_primary DESC, f.created_at`
	return collect(ctx, q, sql, func(r pgx.Rows) (facilityRow, error) {
		var x facilityRow
		return x, r.Scan(&x.companyID, &x.id, &x.facilityType, &x.locality, &x.city, &x.district, &x.stateCode, &x.stateNm)
	}, ids)
}

func filterFacility(f facilityRow, perms permissionSet) PublicFacility {
	pf := PublicFacility{}
	has := func(field string) bool { return perms.has(f.companyID, "facility", f.id, field) }
	if has("facility_type") {
		pf.FacilityType = f.facilityType
	}
	if has("locality") {
		pf.Locality = deref(f.locality)
	}
	if has("city") {
		pf.City = deref(f.city)
	}
	if has("district") {
		pf.District = deref(f.district)
	}
	if has("state_code") && f.stateCode != nil {
		pf.StateCode = *f.stateCode
		pf.StateName = deref(f.stateNm)
	}
	return pf
}

func primaryLocations(ctx context.Context, q DBTX, ids []string, perms permissionSet) (map[string]PublicFacility, error) {
	rows, err := facilitiesFor(ctx, q, ids, true)
	if err != nil {
		return nil, err
	}
	out := map[string]PublicFacility{}
	for _, f := range rows {
		if _, done := out[f.companyID]; done {
			continue
		}
		pf := filterFacility(f, perms)
		if pf.City != "" || pf.StateCode != "" {
			out[f.companyID] = pf
		}
	}
	return out, nil
}

// PublicLookup is the outcome of resolving a public slug.
type PublicLookup struct {
	Company      *PublicCompany
	RedirectSlug string
	Gone         bool
}

// GetPublicCompany resolves a slug to a public profile, a redirect, a 410 (was public, now removed) or not found.
func GetPublicCompany(ctx context.Context, q DBTX, slug string) (PublicLookup, error) {
	raws, err := collect(ctx, q, `SELECT c.id, c.slug, c.display_name, c.verification_status, c.legal_name, c.description,
		c.website_url, c.establishment_year, c.updated_at, c.published_at, c.last_verified_at
		FROM companies c WHERE c.slug = $1 AND `+publishedWhere, scanRaw, slug)
	if err != nil {
		return PublicLookup{}, err
	}
	if len(raws) == 0 {
		var target *string
		err := q.QueryRow(ctx, `SELECT c.slug FROM slug_redirects r JOIN companies c ON c.id = COALESCE(
			(SELECT merged_into_id FROM companies WHERE id = r.company_id), r.company_id)
			WHERE r.old_slug = $1 AND `+publishedWhere, slug).Scan(&target)
		if err == nil && target != nil {
			return PublicLookup{RedirectSlug: *target}, nil
		}
		if err != nil && !IsNotFound(err) {
			return PublicLookup{}, err
		}
		// Merged companies redirect to the surviving record if that one is public.
		err = q.QueryRow(ctx, `SELECT c.slug FROM companies old JOIN companies c ON c.id = old.merged_into_id
			WHERE old.slug = $1 AND `+publishedWhere, slug).Scan(&target)
		if err == nil && target != nil {
			return PublicLookup{RedirectSlug: *target}, nil
		}
		if err != nil && !IsNotFound(err) {
			return PublicLookup{}, err
		}
		var wasPublic bool
		if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM companies WHERE slug = $1 AND published_at IS NOT NULL)`, slug).Scan(&wasPublic); err != nil {
			return PublicLookup{}, err
		}
		return PublicLookup{Gone: wasPublic}, nil
	}
	pc, err := buildPublicCompany(ctx, q, PurposePublic, raws[0])
	if err != nil {
		return PublicLookup{}, err
	}
	return PublicLookup{Company: &pc}, nil
}

func buildPublicCompany(ctx context.Context, q DBTX, purpose Purpose, r rawCompany) (PublicCompany, error) {
	sums, err := buildSummaries(ctx, q, purpose, []rawCompany{r})
	if err != nil {
		return PublicCompany{}, err
	}
	perms, err := loadPermissions(ctx, q, purpose, []string{r.id})
	if err != nil {
		return PublicCompany{}, err
	}
	pc := PublicCompany{PublicCompanySummary: sums[0], PublishedAt: r.publishedAt, LastVerifiedAt: r.lastVerifiedAt,
		Facilities: []PublicFacility{}, Products: []PublicProduct{}, Capabilities: []PublicCapability{},
		Certifications: []PublicCertification{}, Contacts: []PublicContact{}, SourceLabels: []string{}}
	has := func(field string) bool { return perms.has(r.id, "company", r.id, field) }
	if r.legalName != nil && has("legal_name") {
		pc.LegalName = *r.legalName
	}
	if r.website != nil && has("website_url") {
		pc.WebsiteURL = *r.website
	}
	if r.year != nil && has("establishment_year") {
		pc.EstablishmentYear = int(*r.year)
	}
	facs, err := facilitiesFor(ctx, q, []string{r.id}, false)
	if err != nil {
		return pc, err
	}
	for _, f := range facs {
		if pf := filterFacility(f, perms); pf != (PublicFacility{}) {
			pc.Facilities = append(pc.Facilities, pf)
		}
	}
	type prodRow struct {
		id, name    string
		description *string
		catID       *string
		catSlug     *string
		catName     *string
	}
	prods, err := collect(ctx, q, `SELECT p.id, p.name, p.description, cat.id, cat.slug, cat.name FROM products p
		LEFT JOIN categories cat ON cat.id = p.category_id AND cat.active
		WHERE p.company_id = $1 AND p.deleted_at IS NULL ORDER BY p.name`, func(rr pgx.Rows) (prodRow, error) {
		var x prodRow
		return x, rr.Scan(&x.id, &x.name, &x.description, &x.catID, &x.catSlug, &x.catName)
	}, r.id)
	if err != nil {
		return pc, err
	}
	for _, p := range prods {
		if !perms.has(r.id, "product", p.id, "name") {
			continue
		}
		pp := PublicProduct{Name: p.name}
		if p.description != nil && perms.has(r.id, "product", p.id, "description") {
			pp.Description = *p.description
		}
		if p.catID != nil {
			pp.Category = &CategoryRef{ID: *p.catID, Slug: deref(p.catSlug), Name: deref(p.catName)}
		}
		pc.Products = append(pc.Products, pp)
	}
	if pc.Capabilities, err = collect(ctx, q, `SELECT cap.slug, cap.name, cc.verification_status
		FROM company_capabilities cc JOIN capabilities cap ON cap.id = cc.capability_id
		WHERE cc.company_id = $1 AND `+sourceAllows(purpose, "cc.source_id")+` ORDER BY cap.name`,
		func(rr pgx.Rows) (PublicCapability, error) {
			var x PublicCapability
			return x, rr.Scan(&x.Slug, &x.Name, &x.VerificationStatus)
		}, r.id); err != nil {
		return pc, err
	}
	if pc.Certifications, err = collect(ctx, q, `SELECT certification_type, COALESCE(issuer, ''), status FROM certifications ce
		WHERE ce.company_id = $1 AND ce.status NOT IN ('rejected', 'revoked') AND `+sourceAllows(purpose, "ce.source_id")+` ORDER BY certification_type`,
		func(rr pgx.Rows) (PublicCertification, error) {
			var x PublicCertification
			return x, rr.Scan(&x.CertificationType, &x.Issuer, &x.Status)
		}, r.id); err != nil {
		return pc, err
	}
	// Contacts: never individuals, only explicitly public ones with a recorded basis and permitted provenance.
	type contactRow struct {
		id, contactType, channel string
		label                    *string
		value                    string
	}
	contacts, err := collect(ctx, q, `SELECT id, contact_type, channel, label, value FROM contacts
		WHERE company_id = $1 AND deleted_at IS NULL AND contact_type <> 'individual' AND is_public
		AND value IS NOT NULL AND lawful_basis_or_permission_ref IS NOT NULL ORDER BY created_at`,
		func(rr pgx.Rows) (contactRow, error) {
			var x contactRow
			return x, rr.Scan(&x.id, &x.contactType, &x.channel, &x.label, &x.value)
		}, r.id)
	if err != nil {
		return pc, err
	}
	for _, c := range contacts {
		if perms.has(r.id, "contact", c.id, "value") {
			pc.Contacts = append(pc.Contacts, PublicContact{ContactType: c.contactType, Channel: c.channel, Label: deref(c.label), Value: c.value})
		}
	}
	labels, err := collect(ctx, q, `SELECT DISTINCT ds.source_type FROM field_provenance fp
		JOIN data_sources ds ON ds.id = fp.source_id
		WHERE fp.company_id = $1 AND ds.`+purpose.sourceFlag()+` ORDER BY 1`, func(rr pgx.Rows) (string, error) {
		var x string
		return x, rr.Scan(&x)
	}, r.id)
	if err != nil {
		return pc, err
	}
	for _, l := range labels {
		pc.SourceLabels = append(pc.SourceLabels, sourceLabel(l))
	}
	return pc, nil
}

func sourceLabel(sourceType string) string {
	switch sourceType {
	case "self_submission":
		return "Submitted by the business"
	case "authorized_import":
		return "Imported from a dataset with documented reuse rights"
	case "official_site":
		return "Business's official website"
	case "licensed_feed":
		return "Licensed data provider"
	case "manual_verification":
		return "Directory team research"
	default:
		return "Other permitted source"
	}
}

// SitemapEntry is a URL candidate for the sitemap.
type SitemapEntry struct {
	Slug      string     `json:"slug"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// Sitemap lists only indexable public routes: published companies, categories that have
// published companies (or are top-level with a description), and states with published companies.
func Sitemap(ctx context.Context, q DBTX) (map[string][]SitemapEntry, error) {
	scan := func(r pgx.Rows) (SitemapEntry, error) {
		var e SitemapEntry
		return e, r.Scan(&e.Slug, &e.UpdatedAt)
	}
	companies, err := collect(ctx, q, `SELECT c.slug, c.updated_at FROM companies c WHERE `+publishedWhere+` ORDER BY c.slug LIMIT 50000`, scan)
	if err != nil {
		return nil, err
	}
	categories, err := collect(ctx, q, `WITH RECURSIVE tree AS (
			SELECT id, id AS root FROM categories WHERE active
			UNION ALL SELECT ch.id, t.root FROM categories ch JOIN tree t ON ch.parent_id = t.id WHERE ch.active)
		SELECT cat.slug, max(c.updated_at) FROM categories cat
		JOIN tree t ON t.root = cat.id
		JOIN company_categories cc ON cc.category_id = t.id
		JOIN companies c ON c.id = cc.company_id AND `+publishedWhere+`
		WHERE cat.active GROUP BY cat.slug ORDER BY cat.slug`, scan)
	if err != nil {
		return nil, err
	}
	states, err := collect(ctx, q, `SELECT s.slug, max(c.updated_at) FROM indian_states s
		JOIN facilities f ON f.state_code = s.code AND f.deleted_at IS NULL
		JOIN public_field_permissions pf ON pf.entity_type = 'facility' AND pf.entity_id = f.id AND pf.field_name = 'state_code'
		JOIN companies c ON c.id = f.company_id AND `+publishedWhere+`
		GROUP BY s.slug ORDER BY s.slug`, scan)
	if err != nil {
		return nil, err
	}
	return map[string][]SitemapEntry{"companies": companies, "categories": categories, "states": states}, nil
}
