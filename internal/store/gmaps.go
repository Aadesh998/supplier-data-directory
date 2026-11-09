package store

import (
	"context"
	"encoding/json"

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
