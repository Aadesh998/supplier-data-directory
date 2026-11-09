package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/bharatmfg/directory/internal/normalize"
	"github.com/jackc/pgx/v5"
)

// ProviderGmaps jobs are run by the gmaps-worker against the Google Maps scraper service.
const ProviderGmaps = "gmaps_scraper"

// GmapsLead is one business found by the Google Maps scraper.
type GmapsLead struct {
	PlaceKey        string
	Title           string
	Category        string
	Address         string
	Phone           string
	Website         string
	Emails          string
	Latitude        *float64
	Longitude       *float64
	ReviewCount     *int
	ReviewRating    *float64
	BusinessStatus  string
	MapsLink        string
	CompleteAddress json.RawMessage // nil when absent
}

// ExtendLease keeps a long-running job leased to its worker.
func ExtendLease(ctx context.Context, q DBTX, jobID, workerID string) error {
	tag, err := q.Exec(ctx, `UPDATE discovery_jobs SET lease_expires_at = now() + make_interval(secs => $3)
		WHERE id = $1 AND status = 'running' AND locked_by = $2`, jobID, workerID, JobLease.Seconds())
	if err == nil && tag.RowsAffected() == 0 {
		return ErrConflict{Msg: "job is not leased to this worker (lease may have expired)"}
	}
	return err
}

// ReleaseJob hands a claimed job back unchanged (used on shutdown); the attempt is not counted.
func ReleaseJob(ctx context.Context, q DBTX, jobID, workerID string) error {
	_, err := q.Exec(ctx, `UPDATE discovery_jobs SET status = 'pending', attempts = GREATEST(attempts - 1, 0),
		locked_by = NULL, lease_expires_at = NULL WHERE id = $1 AND status = 'running' AND locked_by = $2`, jobID, workerID)
	return err
}

// CompleteGmapsJob saves scraped leads and marks the job done until its next refresh.
func (s *Store) CompleteGmapsJob(ctx context.Context, jobID, workerID string, leads []GmapsLead) (CompleteResult, error) {
	var out CompleteResult
	err := s.InTx(ctx, func(tx pgx.Tx) error {
		var status string
		var lockedBy, category, state *string
		var location string
		if err := tx.QueryRow(ctx, `SELECT status, locked_by, category_slug, state_code, location FROM discovery_jobs
			WHERE id = $1 FOR UPDATE`, jobID).Scan(&status, &lockedBy, &category, &state, &location); err != nil {
			return err
		}
		if status != "running" || lockedBy == nil || *lockedBy != workerID {
			return ErrConflict{Msg: "job is not leased to this worker (lease may have expired)"}
		}
		out.Received = len(leads)
		for _, l := range leads {
			var addr any
			if len(l.CompleteAddress) > 0 {
				addr = string(l.CompleteAddress)
			}
			var inserted bool
			if err := tx.QueryRow(ctx, `INSERT INTO gmaps_leads (place_key, first_job_id, last_job_id, category_slug, state_code,
				location, title, category, address, phone, website, emails, latitude, longitude, review_count, review_rating,
				business_status, maps_link, complete_address)
				VALUES ($1,$2,$2,$3,$4,$5,$6,NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),NULLIF($11,''),$12,$13,$14,$15,
				  NULLIF($16,''),NULLIF($17,''),$18::jsonb)
				ON CONFLICT (place_key) DO UPDATE SET last_job_id = EXCLUDED.last_job_id, last_seen_at = now(),
				  seen_count = gmaps_leads.seen_count + 1, title = EXCLUDED.title,
				  category = COALESCE(EXCLUDED.category, gmaps_leads.category),
				  address = COALESCE(EXCLUDED.address, gmaps_leads.address),
				  phone = COALESCE(EXCLUDED.phone, gmaps_leads.phone),
				  website = COALESCE(EXCLUDED.website, gmaps_leads.website),
				  emails = COALESCE(EXCLUDED.emails, gmaps_leads.emails),
				  latitude = COALESCE(EXCLUDED.latitude, gmaps_leads.latitude),
				  longitude = COALESCE(EXCLUDED.longitude, gmaps_leads.longitude),
				  review_count = COALESCE(EXCLUDED.review_count, gmaps_leads.review_count),
				  review_rating = COALESCE(EXCLUDED.review_rating, gmaps_leads.review_rating),
				  business_status = COALESCE(EXCLUDED.business_status, gmaps_leads.business_status),
				  maps_link = COALESCE(EXCLUDED.maps_link, gmaps_leads.maps_link),
				  complete_address = COALESCE(EXCLUDED.complete_address, gmaps_leads.complete_address),
				  category_slug = COALESCE(gmaps_leads.category_slug, EXCLUDED.category_slug),
				  state_code = COALESCE(gmaps_leads.state_code, EXCLUDED.state_code),
				  location = COALESCE(gmaps_leads.location, EXCLUDED.location)
				RETURNING (xmax = 0)`,
				l.PlaceKey, jobID, category, state, location, l.Title, l.Category, l.Address, l.Phone, l.Website, l.Emails,
				l.Latitude, l.Longitude, l.ReviewCount, l.ReviewRating, l.BusinessStatus, l.MapsLink, addr).Scan(&inserted); err != nil {
				return err
			}
			if inserted {
				out.New++
			}
		}
		_, err := tx.Exec(ctx, `UPDATE discovery_jobs SET status = 'done', attempts = 0, locked_by = NULL, lease_expires_at = NULL,
			last_error = NULL, results_count = $2, total_results = total_results + $3,
			next_run_after = now() + make_interval(days => refresh_interval_days) WHERE id = $1`, jobID, out.Received, out.New)
		return err
	})
	return out, err
}

// GmapsLeadView is a gmaps_leads row for the admin review page.
type GmapsLeadView struct {
	ID                string    `json:"id"`
	Title             string    `json:"title"`
	Category          *string   `json:"category"`
	Address           *string   `json:"address"`
	Phone             *string   `json:"phone"`
	Website           *string   `json:"website"`
	Emails            *string   `json:"emails"`
	ReviewCount       *int      `json:"review_count"`
	ReviewRating      *float64  `json:"review_rating"`
	BusinessStatus    *string   `json:"business_status"`
	MapsLink          *string   `json:"maps_link"`
	CategorySlug      *string   `json:"category_slug"`
	StateCode         *string   `json:"state_code"`
	Location          *string   `json:"location"`
	SearchQuery       *string   `json:"search_query"`
	Status            string    `json:"status"`
	PromotedCompanyID *string   `json:"promoted_company_id"`
	SeenCount         int       `json:"seen_count"`
	FirstSeenAt       time.Time `json:"first_seen_at"`
	LastSeenAt        time.Time `json:"last_seen_at"`
}

const gmapsLeadCols = `l.id, l.title, l.category, l.address, l.phone, l.website, l.emails, l.review_count, l.review_rating::float8,
	l.business_status, l.maps_link, l.category_slug, l.state_code, l.location, j.search_query, l.status, l.promoted_company_id,
	l.seen_count, l.first_seen_at, l.last_seen_at`

func scanGmapsLead(row pgx.Row) (GmapsLeadView, error) {
	var x GmapsLeadView
	err := row.Scan(&x.ID, &x.Title, &x.Category, &x.Address, &x.Phone, &x.Website, &x.Emails, &x.ReviewCount, &x.ReviewRating,
		&x.BusinessStatus, &x.MapsLink, &x.CategorySlug, &x.StateCode, &x.Location, &x.SearchQuery, &x.Status, &x.PromotedCompanyID,
		&x.SeenCount, &x.FirstSeenAt, &x.LastSeenAt)
	return x, err
}

// GmapsLeadFilter narrows ListGmapsLeads. Empty fields match everything.
type GmapsLeadFilter struct {
	Status, Category, State, Search, JobKey string
	HasPhone, HasWebsite                    bool
}

// ListGmapsLeads lists Google Maps leads for review, newest first.
func ListGmapsLeads(ctx context.Context, q DBTX, f GmapsLeadFilter, limit, offset int) ([]GmapsLeadView, int, error) {
	where := `($1 = '' OR l.status = $1) AND ($2 = '' OR l.category_slug = $2) AND ($3 = '' OR l.state_code = upper($3))
		AND ($4 = '' OR l.title ILIKE '%' || $4 || '%' OR l.address ILIKE '%' || $4 || '%' OR l.phone ILIKE '%' || $4 || '%')
		AND ($5 = '' OR j.job_key = $5)
		AND (NOT $6 OR COALESCE(l.phone, '') <> '') AND (NOT $7 OR COALESCE(l.website, '') <> '')`
	args := []any{f.Status, f.Category, f.State, f.Search, f.JobKey, f.HasPhone, f.HasWebsite}
	from := ` FROM gmaps_leads l LEFT JOIN discovery_jobs j ON j.id = l.first_job_id WHERE `
	var total int
	if err := q.QueryRow(ctx, `SELECT count(*)`+from+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	out, err := collect(ctx, q, `SELECT `+gmapsLeadCols+from+where+` ORDER BY l.last_seen_at DESC, l.title LIMIT $8 OFFSET $9`,
		func(r pgx.Rows) (GmapsLeadView, error) { return scanGmapsLead(r) }, append(args, limit, offset)...)
	return out, total, err
}

func getGmapsLead(ctx context.Context, q DBTX, id string, lock bool) (GmapsLeadView, error) {
	sql := `SELECT ` + gmapsLeadCols + ` FROM gmaps_leads l LEFT JOIN discovery_jobs j ON j.id = l.first_job_id WHERE l.id = $1`
	if lock {
		sql += ` FOR UPDATE OF l`
	}
	return scanGmapsLead(q.QueryRow(ctx, sql, id))
}

// SetGmapsLeadStatus marks a lead contacted or dismissed (or back to new). Promoted leads stay promoted.
func SetGmapsLeadStatus(ctx context.Context, q DBTX, a Actor, id, status string) (GmapsLeadView, error) {
	tag, err := q.Exec(ctx, `UPDATE gmaps_leads SET status = $2 WHERE id = $1 AND status <> 'promoted'`, id, status)
	if err != nil {
		return GmapsLeadView{}, err
	}
	if tag.RowsAffected() == 0 {
		return GmapsLeadView{}, ErrConflict{Msg: "lead not found or already promoted"}
	}
	if err := Audit(ctx, q, a, "gmaps.lead_"+status, "gmaps_lead", &id, nil, nil); err != nil {
		return GmapsLeadView{}, err
	}
	return getGmapsLead(ctx, q, id, false)
}

// gmapsSource returns (creating on first use) the data source for promoted Google Maps leads.
// Internal use only: Google Maps' terms don't grant reuse, so an owner must decide before anything is public.
func gmapsSource(ctx context.Context, tx DBTX, a Actor) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `SELECT id FROM data_sources WHERE provider = 'google_maps_scraper' AND source_type = 'public_web_listing'
		ORDER BY created_at LIMIT 1`).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !IsNotFound(err) {
		return "", err
	}
	src, err := CreateSource(ctx, tx, a, SourceInput{SourceType: "public_web_listing", Provider: "google_maps_scraper",
		SourceURL: ptr("https://maps.google.com"), AllowedInternalUse: ptr(true),
		Notes: ptr("Businesses found on Google Maps by the gmaps worker. Internal use only: confirm details with each business " +
			"(or have it register) before enabling public display.")})
	return src.ID, err
}

// PromoteGmapsLead turns a lead into a private draft company (internal-only source) with its category,
// location, website and phone, and flags possible duplicates.
func (s *Store) PromoteGmapsLead(ctx context.Context, a Actor, id string) (string, error) {
	var companyID string
	err := s.InTx(ctx, func(tx pgx.Tx) error {
		l, err := getGmapsLead(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if l.Status == "promoted" {
			return ErrConflict{Msg: "lead already promoted"}
		}
		var addr struct {
			City       string `json:"city"`
			PostalCode string `json:"postal_code"`
			Street     string `json:"street"`
			Borough    string `json:"borough"`
		}
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT complete_address FROM gmaps_leads WHERE id = $1`, id).Scan(&raw); err != nil {
			return err
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &addr)
		}
		src, err := gmapsSource(ctx, tx, a)
		if err != nil {
			return err
		}
		name := truncateRunes(l.Title, 300)
		in := CompanyInput{SourceID: &src, DisplayName: &name}
		if l.Website != nil {
			if canon, _, ok := normalize.WebsiteURL(*l.Website); ok {
				in.WebsiteURL = &canon
			}
		}
		if l.CategorySlug != nil {
			var catID string
			if err := tx.QueryRow(ctx, `SELECT id FROM categories WHERE slug = $1`, *l.CategorySlug).Scan(&catID); err == nil {
				in.CategoryIDs = &[]string{catID}
				in.PrimaryCategoryID = &catID
			}
		}
		if companyID, err = createCompanyTx(ctx, tx, a, in, "draft"); err != nil {
			return err
		}
		city := addr.City
		if city == "" && l.Location != nil {
			city = *l.Location
		}
		fi := FacilityInput{SourceID: src, FacilityType: "other", City: strOrNil(truncateRunes(city, 120)), StateCode: l.StateCode,
			Locality: strOrNil(truncateRunes(addr.Borough, 120)), IsPrimary: true}
		if l.Address != nil {
			fi.AddressLine1 = strOrNil(truncateRunes(*l.Address, 200))
		}
		if pc, ok := normalize.PostalCode(addr.PostalCode); ok {
			fi.PostalCode = &pc
		}
		if _, err := insertFacility(ctx, tx, companyID, fi); err != nil {
			return err
		}
		// Phones are stored encrypted and never public, like imported phone numbers.
		if l.Phone != nil {
			if phone, ok := normalize.Phone(*l.Phone); ok {
				enc, err := s.Box.Encrypt(phone)
				if err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `INSERT INTO contacts (company_id, contact_type, channel, value_encrypted, value_hash, source_id,
					lawful_basis_or_permission_ref) VALUES ($1, 'individual', 'phone', $2, $3, $4, $5)`,
					companyID, enc, s.Box.Hash(phone), src, "gmaps_lead:"+id); err != nil {
					return err
				}
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE gmaps_leads SET status = 'promoted', promoted_company_id = $2 WHERE id = $1`, id, companyID); err != nil {
			return err
		}
		if _, err := FindDuplicatesFor(ctx, tx, companyID); err != nil {
			return err
		}
		return Audit(ctx, tx, a, "gmaps.lead_promote", "gmaps_lead", &id, nil, map[string]any{"company_id": companyID})
	})
	return companyID, err
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
