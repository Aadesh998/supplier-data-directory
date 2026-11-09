package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bharatmfg/directory/internal/normalize"
)

// MaxJobRows bounds one jobs CSV upload.
const MaxJobRows = 50000

// JobLease is how long a worker owns a claimed job before it can be reclaimed.
const JobLease = 10 * time.Minute

// Discovery providers.
const (
	ProviderOSM          = "osm_overpass"
	ProviderGooglePlaces = "google_places_ids"
)

// JobBatch summarises a jobs CSV upload.
type JobBatch struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"`
	Inserted  int       `json:"inserted"`
	Updated   int       `json:"updated"`
	Skipped   int       `json:"skipped"`
	Errors    []string  `json:"errors"`
	CreatedAt time.Time `json:"created_at"`
}

// DiscoveryJob is one search job.
type DiscoveryJob struct {
	ID                  string     `json:"id"`
	JobKey              string     `json:"job_key"`
	Provider            string     `json:"provider"`
	Priority            int16      `json:"priority"`
	SectorFamily        *string    `json:"sector_family"`
	CategorySlug        *string    `json:"category_slug"`
	Keyword             string     `json:"keyword"`
	SearchQuery         string     `json:"search_query"`
	OSMNameRegex        *string    `json:"osm_name_regex"`
	Location            string     `json:"location"`
	StateCode           *string    `json:"state_code"`
	Latitude            float64    `json:"latitude"`
	Longitude           float64    `json:"longitude"`
	RadiusKM            float64    `json:"radius_km"`
	IsCluster           bool       `json:"is_cluster"`
	Status              string     `json:"status"`
	Attempts            int        `json:"attempts"`
	MaxAttempts         int        `json:"max_attempts"`
	RefreshIntervalDays int        `json:"refresh_interval_days"`
	LockedBy            *string    `json:"locked_by"`
	LeaseExpiresAt      *time.Time `json:"lease_expires_at"`
	LastRunAt           *time.Time `json:"last_run_at"`
	NextRunAfter        *time.Time `json:"next_run_after"`
	LastError           *string    `json:"last_error"`
	ResultsCount        *int       `json:"results_count"`
	TotalResults        int        `json:"total_results"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

const jobCols = `id, job_key, provider, priority, sector_family, category_slug, keyword, search_query, osm_name_regex,
	location, state_code, latitude::float8, longitude::float8, radius_km::float8, is_cluster, status, attempts, max_attempts,
	refresh_interval_days, locked_by, lease_expires_at, last_run_at, next_run_after, last_error, results_count, total_results, updated_at`

func scanJob(r pgx.Row) (DiscoveryJob, error) {
	var j DiscoveryJob
	err := r.Scan(&j.ID, &j.JobKey, &j.Provider, &j.Priority, &j.SectorFamily, &j.CategorySlug, &j.Keyword, &j.SearchQuery,
		&j.OSMNameRegex, &j.Location, &j.StateCode, &j.Latitude, &j.Longitude, &j.RadiusKM, &j.IsCluster, &j.Status,
		&j.Attempts, &j.MaxAttempts, &j.RefreshIntervalDays, &j.LockedBy, &j.LeaseExpiresAt, &j.LastRunAt, &j.NextRunAfter,
		&j.LastError, &j.ResultsCount, &j.TotalResults, &j.UpdatedAt)
	return j, err
}

// jobRow is a parsed, validated CSV row.
type jobRow struct {
	key, provider, sectorFamily, categorySlug, keyword, query, regex, location, stateCode string
	priority, tier, refreshDays                                                           int
	lat, lng, radius                                                                      float64
	cluster                                                                               bool
}

var requiredJobColumns = []string{"job_id", "keyword", "search_query", "location", "latitude", "longitude", "radius_km"}

func parseJobsCSV(data []byte, validCategories, validStates map[string]bool) ([]jobRow, []string, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("could not read header: %w", err)
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	var missing []string
	for _, c := range requiredJobColumns {
		if _, ok := idx[c]; !ok {
			missing = append(missing, c)
		}
	}
	if len(missing) > 0 {
		return nil, nil, fmt.Errorf("missing required column(s): %s", strings.Join(missing, ", "))
	}
	get := func(rec []string, col string) string {
		if i, ok := idx[col]; ok && i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}
	var rows []jobRow
	var problems []string
	seen := map[string]bool{}
	line := 1
	for {
		rec, err := r.Read()
		line++
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("malformed CSV near line %d: %w", line, err)
		}
		if len(rows) >= MaxJobRows {
			return nil, nil, fmt.Errorf("more than %d jobs in one file; split it", MaxJobRows)
		}
		bad := func(msg string) { problems = append(problems, fmt.Sprintf("line %d: %s", line, msg)) }
		j := jobRow{key: get(rec, "job_id"), keyword: get(rec, "keyword"), query: get(rec, "search_query"),
			location: get(rec, "location"), sectorFamily: get(rec, "sector_family"), regex: get(rec, "osm_name_regex"),
			provider: get(rec, "provider"), priority: 3, refreshDays: 30}
		if j.key == "" || len(j.key) > 64 || j.keyword == "" || j.query == "" || j.location == "" {
			bad("job_id, keyword, search_query and location are required (job_id max 64 chars)")
			continue
		}
		if seen[j.key] {
			bad("duplicate job_id " + j.key)
			continue
		}
		seen[j.key] = true
		if j.provider == "" {
			j.provider = ProviderOSM
		}
		if j.provider != ProviderOSM && j.provider != ProviderGooglePlaces && j.provider != ProviderGmaps {
			bad("provider must be osm_overpass, google_places_ids or gmaps_scraper")
			continue
		}
		var errLat, errLng, errRad error
		j.lat, errLat = strconv.ParseFloat(get(rec, "latitude"), 64)
		j.lng, errLng = strconv.ParseFloat(get(rec, "longitude"), 64)
		j.radius, errRad = strconv.ParseFloat(get(rec, "radius_km"), 64)
		if errLat != nil || errLng != nil || errRad != nil || j.lat < -90 || j.lat > 90 || j.lng < -180 || j.lng > 180 || j.radius <= 0 || j.radius > 50 {
			bad("latitude/longitude/radius_km invalid (radius must be 0-50 km)")
			continue
		}
		if p, err := strconv.Atoi(get(rec, "priority")); err == nil && p >= 1 && p <= 9 {
			j.priority = p
		}
		if t, err := strconv.Atoi(get(rec, "location_tier")); err == nil {
			j.tier = t
		}
		if d, err := strconv.Atoi(get(rec, "refresh_interval_days")); err == nil && d >= 1 && d <= 365 {
			j.refreshDays = d
		}
		j.cluster = strings.EqualFold(get(rec, "is_cluster"), "true")
		if c := get(rec, "category_slug"); c != "" {
			if validCategories[c] {
				j.categorySlug = c
			} else {
				bad("unknown category_slug " + c + " (stored without category)")
			}
		}
		if s := get(rec, "state_code"); s != "" {
			if code, ok := normalize.StateCode(s); ok && validStates[code] {
				j.stateCode = code
			} else {
				bad("unknown state_code " + s + " (stored without state)")
			}
		}
		rows = append(rows, j)
	}
	if len(rows) == 0 {
		return nil, problems, errors.New("no valid job rows found")
	}
	return rows, problems, nil
}

// UploadJobs upserts jobs by job_id. Existing jobs keep their run state (status, attempts,
// next run) unless resetState is true; their search definition is updated.
func (s *Store) UploadJobs(ctx context.Context, a Actor, filename string, data []byte, resetState bool) (JobBatch, error) {
	cats, err := collect(ctx, s.Pool, `SELECT slug FROM categories`, func(r pgx.Rows) (string, error) {
		var x string
		return x, r.Scan(&x)
	})
	if err != nil {
		return JobBatch{}, err
	}
	validCats := map[string]bool{}
	for _, c := range cats {
		validCats[c] = true
	}
	validStates := map[string]bool{}
	for code := range normalize.States {
		validStates[code] = true
	}
	rows, problems, err := parseJobsCSV(data, validCats, validStates)
	if err != nil {
		return JobBatch{}, ErrInvalid{Msg: err.Error(), Fields: map[string]string{"file": err.Error()}}
	}
	sum := sha256.Sum256(data)
	b := JobBatch{Filename: filename, Errors: problems}
	if b.Errors == nil {
		b.Errors = []string{}
	}
	if len(b.Errors) > 200 {
		b.Errors = append(b.Errors[:200], fmt.Sprintf("… and %d more", len(problems)-200))
	}
	err = s.InTx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO discovery_job_batches (filename, file_hash, uploaded_by) VALUES ($1,$2,$3)
			RETURNING id, created_at`, filename, hex.EncodeToString(sum[:]), a.ID).Scan(&b.ID, &b.CreatedAt); err != nil {
			return err
		}
		batch := &pgx.Batch{}
		for _, j := range rows {
			batch.Queue(`INSERT INTO discovery_jobs (job_key, batch_id, provider, priority, sector_family, category_slug, keyword,
				search_query, osm_name_regex, location, state_code, latitude, longitude, radius_km, location_tier, is_cluster,
				refresh_interval_days)
				VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),$7,$8,NULLIF($9,''),$10,NULLIF($11,''),$12,$13,$14,$15,$16,$17)
				ON CONFLICT (job_key) DO UPDATE SET batch_id = EXCLUDED.batch_id, provider = EXCLUDED.provider,
				  priority = EXCLUDED.priority, sector_family = EXCLUDED.sector_family, category_slug = EXCLUDED.category_slug,
				  keyword = EXCLUDED.keyword, search_query = EXCLUDED.search_query, osm_name_regex = EXCLUDED.osm_name_regex,
				  location = EXCLUDED.location, state_code = EXCLUDED.state_code, latitude = EXCLUDED.latitude,
				  longitude = EXCLUDED.longitude, radius_km = EXCLUDED.radius_km, location_tier = EXCLUDED.location_tier,
				  is_cluster = EXCLUDED.is_cluster, refresh_interval_days = EXCLUDED.refresh_interval_days,
				  status = CASE WHEN $18 AND discovery_jobs.status <> 'running' THEN 'pending' ELSE discovery_jobs.status END,
				  attempts = CASE WHEN $18 AND discovery_jobs.status <> 'running' THEN 0 ELSE discovery_jobs.attempts END,
				  next_run_after = CASE WHEN $18 AND discovery_jobs.status <> 'running' THEN NULL ELSE discovery_jobs.next_run_after END
				RETURNING (xmax = 0)`,
				j.key, b.ID, j.provider, j.priority, j.sectorFamily, j.categorySlug, j.keyword, j.query, j.regex, j.location,
				j.stateCode, j.lat, j.lng, j.radius, j.tier, j.cluster, j.refreshDays, resetState)
		}
		br := tx.SendBatch(ctx, batch)
		for range rows {
			var inserted bool
			if err := br.QueryRow().Scan(&inserted); err != nil {
				br.Close()
				return err
			}
			if inserted {
				b.Inserted++
			} else {
				b.Updated++
			}
		}
		if err := br.Close(); err != nil {
			return err
		}
		b.Skipped = len(problems) - countStored(problems)
		if _, err := tx.Exec(ctx, `UPDATE discovery_job_batches SET inserted = $2, updated = $3, skipped = $4, errors = $5 WHERE id = $1`,
			b.ID, b.Inserted, b.Updated, b.Skipped, mustJSON(b.Errors)); err != nil {
			return err
		}
		return Audit(ctx, tx, a, "discovery.jobs_upload", "discovery_job_batch", &b.ID, nil,
			map[string]any{"filename": filename, "inserted": b.Inserted, "updated": b.Updated, "skipped": b.Skipped, "reset": resetState})
	})
	return b, err
}

// countStored counts warnings for rows that were still stored (unknown category/state).
func countStored(problems []string) int {
	n := 0
	for _, p := range problems {
		if strings.Contains(p, "(stored without") {
			n++
		}
	}
	return n
}

// JobFilter filters the admin job list.
type JobFilter struct {
	Status, Provider, StateCode, CategorySlug, Q string
	Priority                                     int
}

// ListJobs lists jobs, highest priority first.
func ListJobs(ctx context.Context, q DBTX, f JobFilter, limit, offset int) ([]DiscoveryJob, int, error) {
	where := `($1 = '' OR status = $1) AND ($2 = '' OR provider = $2) AND ($3 = '' OR state_code = $3)
		AND ($4 = '' OR category_slug = $4) AND ($5 = 0 OR priority = $5)
		AND ($6 = '' OR search_query ILIKE '%' || $6 || '%' OR job_key = $6)`
	args := []any{f.Status, f.Provider, strings.ToUpper(f.StateCode), f.CategorySlug, f.Priority, f.Q}
	var total int
	if err := q.QueryRow(ctx, `SELECT count(*) FROM discovery_jobs WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	out, err := collect(ctx, q, `SELECT `+jobCols+` FROM discovery_jobs WHERE `+where+
		` ORDER BY priority, next_run_after NULLS FIRST, job_key LIMIT $7 OFFSET $8`,
		func(r pgx.Rows) (DiscoveryJob, error) { return scanJob(r) }, append(args, limit, offset)...)
	return out, total, err
}

// JobStats summarises the queue.
type JobStats struct {
	ByStatus     map[string]int `json:"by_status"`
	DueNow       int            `json:"due_now"`
	Results      map[string]int `json:"results_by_status"`
	LastBatch    *JobBatch      `json:"last_batch"`
	LastRunAt    *time.Time     `json:"last_run_at"`
	TotalResults int            `json:"total_results"`
}

// dueCondition selects jobs a worker may claim right now.
const dueCondition = `((status = 'pending' AND (next_run_after IS NULL OR next_run_after <= now()))
	OR (status = 'done' AND next_run_after <= now())
	OR (status = 'running' AND lease_expires_at < now()))`

// GetJobStats computes queue statistics.
func GetJobStats(ctx context.Context, q DBTX) (JobStats, error) {
	st := JobStats{ByStatus: map[string]int{}, Results: map[string]int{}}
	type kv struct {
		k string
		n int
	}
	scan := func(r pgx.Rows) (kv, error) {
		var x kv
		return x, r.Scan(&x.k, &x.n)
	}
	rows, err := collect(ctx, q, `SELECT status, count(*) FROM discovery_jobs GROUP BY status`, scan)
	if err != nil {
		return st, err
	}
	for _, r := range rows {
		st.ByStatus[r.k] = r.n
	}
	if rows, err = collect(ctx, q, `SELECT status, count(*) FROM discovery_results GROUP BY status`, scan); err != nil {
		return st, err
	}
	for _, r := range rows {
		st.Results[r.k] = r.n
		st.TotalResults += r.n
	}
	if err := q.QueryRow(ctx, `SELECT count(*) FILTER (WHERE `+dueCondition+`), max(last_run_at) FROM discovery_jobs`).Scan(&st.DueNow, &st.LastRunAt); err != nil {
		return st, err
	}
	var b JobBatch
	err = q.QueryRow(ctx, `SELECT id, filename, inserted, updated, skipped, errors, created_at FROM discovery_job_batches
		ORDER BY created_at DESC LIMIT 1`).Scan(&b.ID, &b.Filename, &b.Inserted, &b.Updated, &b.Skipped, &b.Errors, &b.CreatedAt)
	if err == nil {
		st.LastBatch = &b
	} else if !IsNotFound(err) {
		return st, err
	}
	return st, nil
}

// JobAction is a bulk admin action on jobs.
type JobAction struct {
	Action string   `json:"action" validate:"required,oneof=pause resume retry_failed run_now"`
	Status string   `json:"status" validate:"omitempty,oneof=pending running done failed paused"`
	JobIDs []string `json:"job_ids" validate:"omitempty,max=1000,dive,uuid"`
}

// ApplyJobAction pauses, resumes, retries or prioritises jobs (by ids, or by status filter).
func ApplyJobAction(ctx context.Context, q DBTX, a Actor, in JobAction) (int64, error) {
	var set, cond string
	switch in.Action {
	case "pause":
		set, cond = `status = 'paused', locked_by = NULL, lease_expires_at = NULL`, `status IN ('pending', 'done', 'failed')`
	case "resume":
		set, cond = `status = 'pending', next_run_after = NULL`, `status = 'paused'`
	case "retry_failed":
		set, cond = `status = 'pending', attempts = 0, next_run_after = NULL, last_error = NULL`, `status = 'failed'`
	case "run_now":
		set, cond = `next_run_after = NULL, status = CASE WHEN status = 'done' THEN 'pending' ELSE status END`, `status IN ('pending', 'done')`
	}
	sql := `UPDATE discovery_jobs SET ` + set + ` WHERE ` + cond + ` AND ($1 = '' OR status = $1) AND (cardinality($2::uuid[]) = 0 OR id = ANY($2::uuid[]))`
	ids := in.JobIDs
	if ids == nil {
		ids = []string{}
	}
	tag, err := q.Exec(ctx, sql, in.Status, ids)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), Audit(ctx, q, a, "discovery.jobs_"+in.Action, "discovery_job", nil, nil,
		map[string]any{"affected": tag.RowsAffected(), "status_filter": in.Status, "ids": len(ids)})
}

// ClaimJobs leases up to limit due jobs to a worker. Safe for concurrent workers.
func ClaimJobs(ctx context.Context, q DBTX, workerID string, limit int, providers []string) ([]DiscoveryJob, error) {
	if len(providers) == 0 {
		providers = []string{ProviderOSM}
	}
	return collect(ctx, q, `WITH due AS (
			SELECT id FROM discovery_jobs WHERE provider = ANY($3) AND `+dueCondition+`
			ORDER BY priority, next_run_after NULLS FIRST, created_at
			LIMIT $2 FOR UPDATE SKIP LOCKED)
		UPDATE discovery_jobs j SET status = 'running', locked_by = $1, lease_expires_at = now() + make_interval(secs => $4),
			attempts = j.attempts + 1, last_run_at = now()
		FROM due WHERE j.id = due.id RETURNING `+prefixCols("j.", jobCols),
		func(r pgx.Rows) (DiscoveryJob, error) { return scanJob(r) }, workerID, limit, providers, JobLease.Seconds())
}

func prefixCols(prefix, cols string) string {
	parts := strings.Split(cols, ",")
	for i, p := range parts {
		parts[i] = prefix + strings.TrimSpace(p)
	}
	return strings.Join(parts, ", ")
}

// JobResult is one item a worker found.
type JobResult struct {
	ProviderRef string            `json:"provider_ref" validate:"required,max=512"`
	Name        string            `json:"name" validate:"max=300"`
	Latitude    *float64          `json:"latitude" validate:"omitempty,gte=-90,lte=90"`
	Longitude   *float64          `json:"longitude" validate:"omitempty,gte=-180,lte=180"`
	Tags        map[string]string `json:"tags" validate:"omitempty,max=60"`
}

// CompleteResult reports what a completion did.
type CompleteResult struct {
	Received int `json:"received"`
	New      int `json:"new"`
}

// keepOSMTags is the allowlist of OpenStreetMap tags worth keeping (business facts, no personal data).
var keepOSMTags = map[string]bool{
	"name": true, "operator": true, "brand": true, "website": true, "contact:website": true, "industrial": true,
	"man_made": true, "craft": true, "shop": true, "office": true, "product": true, "landuse": true, "building": true,
	"addr:street": true, "addr:city": true, "addr:district": true, "addr:state": true, "addr:postcode": true,
	"addr:suburb": true, "addr:place": true, "opening_hours": false,
}

// CompleteJob stores a worker's results and schedules the job's next refresh.
func (s *Store) CompleteJob(ctx context.Context, jobID, workerID string, results []JobResult) (CompleteResult, error) {
	var out CompleteResult
	err := s.InTx(ctx, func(tx pgx.Tx) error {
		var provider, status string
		var lockedBy *string
		var category *string
		if err := tx.QueryRow(ctx, `SELECT provider, status, locked_by, category_slug FROM discovery_jobs WHERE id = $1 FOR UPDATE`, jobID).
			Scan(&provider, &status, &lockedBy, &category); err != nil {
			return err
		}
		if status != "running" || lockedBy == nil || *lockedBy != workerID {
			return ErrConflict{Msg: "job is not leased to this worker (lease may have expired)"}
		}
		out.Received = len(results)
		for _, r := range results {
			name, lat, lng := strOrNil(normalize.Whitespace(r.Name)), r.Latitude, r.Longitude
			tags := map[string]string{}
			if provider == ProviderGooglePlaces {
				name, lat, lng = nil, nil, nil // only the place ID may be stored
			} else {
				for k, v := range r.Tags {
					if keepOSMTags[k] && len(v) <= 300 {
						tags[k] = v
					}
				}
			}
			var inserted bool
			if err := tx.QueryRow(ctx, `INSERT INTO discovery_results (provider, provider_ref, first_job_id, last_job_id, category_slug,
				name, latitude, longitude, tags) VALUES ($1,$2,$3,$3,$4,$5,$6,$7,$8)
				ON CONFLICT (provider, provider_ref) DO UPDATE SET last_job_id = EXCLUDED.last_job_id, last_seen_at = now(),
				  seen_count = discovery_results.seen_count + 1, name = COALESCE(EXCLUDED.name, discovery_results.name),
				  latitude = COALESCE(EXCLUDED.latitude, discovery_results.latitude),
				  longitude = COALESCE(EXCLUDED.longitude, discovery_results.longitude),
				  tags = CASE WHEN EXCLUDED.tags = '{}'::jsonb THEN discovery_results.tags ELSE EXCLUDED.tags END,
				  category_slug = COALESCE(discovery_results.category_slug, EXCLUDED.category_slug)
				RETURNING (xmax = 0)`, provider, r.ProviderRef, jobID, category, name, lat, lng, tags).Scan(&inserted); err != nil {
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

// FailJob records a failure with exponential backoff; after max_attempts the job is marked failed.
func FailJob(ctx context.Context, q DBTX, jobID, workerID, message string) (DiscoveryJob, error) {
	if len(message) > 1000 {
		message = message[:1000]
	}
	j, err := scanJob(q.QueryRow(ctx, `UPDATE discovery_jobs SET
		status = CASE WHEN attempts >= max_attempts THEN 'failed' ELSE 'pending' END,
		next_run_after = now() + make_interval(mins => 5 * power(2, LEAST(attempts, 8))::int),
		last_error = $3, locked_by = NULL, lease_expires_at = NULL
		WHERE id = $1 AND status = 'running' AND locked_by = $2 RETURNING `+jobCols, jobID, workerID, message))
	if IsNotFound(err) {
		return j, ErrConflict{Msg: "job is not leased to this worker (lease may have expired)"}
	}
	return j, err
}

// DiscoveryResult is a lead found by a worker.
type DiscoveryResult struct {
	ID               string            `json:"id"`
	Provider         string            `json:"provider"`
	ProviderRef      string            `json:"provider_ref"`
	CategorySlug     *string           `json:"category_slug"`
	Name             *string           `json:"name"`
	Latitude         *float64          `json:"latitude"`
	Longitude        *float64          `json:"longitude"`
	Tags             map[string]string `json:"tags"`
	Status           string            `json:"status"`
	MatchedCompanyID *string           `json:"matched_company_id"`
	SeenCount        int               `json:"seen_count"`
	JobLocation      *string           `json:"job_location"`
	JobStateCode     *string           `json:"job_state_code"`
	FirstSeenAt      time.Time         `json:"first_seen_at"`
	LastSeenAt       time.Time         `json:"last_seen_at"`
}

const resultCols = `r.id, r.provider, r.provider_ref, r.category_slug, r.name, r.latitude::float8, r.longitude::float8, r.tags,
	r.status, r.matched_company_id, r.seen_count, j.location, j.state_code, r.first_seen_at, r.last_seen_at`

func scanResult(row pgx.Row) (DiscoveryResult, error) {
	var x DiscoveryResult
	err := row.Scan(&x.ID, &x.Provider, &x.ProviderRef, &x.CategorySlug, &x.Name, &x.Latitude, &x.Longitude, &x.Tags,
		&x.Status, &x.MatchedCompanyID, &x.SeenCount, &x.JobLocation, &x.JobStateCode, &x.FirstSeenAt, &x.LastSeenAt)
	return x, err
}

// ListResults lists discovery results for review.
func ListResults(ctx context.Context, q DBTX, status, provider, category, search string, limit, offset int) ([]DiscoveryResult, int, error) {
	where := `($1 = '' OR r.status = $1) AND ($2 = '' OR r.provider = $2) AND ($3 = '' OR r.category_slug = $3)
		AND ($4 = '' OR r.name ILIKE '%' || $4 || '%' OR r.provider_ref = $4)`
	args := []any{status, provider, category, search}
	var total int
	if err := q.QueryRow(ctx, `SELECT count(*) FROM discovery_results r WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	out, err := collect(ctx, q, `SELECT `+resultCols+` FROM discovery_results r LEFT JOIN discovery_jobs j ON j.id = r.last_job_id
		WHERE `+where+` ORDER BY r.last_seen_at DESC LIMIT $5 OFFSET $6`,
		func(r pgx.Rows) (DiscoveryResult, error) { return scanResult(r) }, append(args, limit, offset)...)
	return out, total, err
}

// SetResultStatus marks a result contacted or dismissed (or back to new).
func SetResultStatus(ctx context.Context, q DBTX, a Actor, id, status string) (DiscoveryResult, error) {
	tag, err := q.Exec(ctx, `UPDATE discovery_results SET status = $2 WHERE id = $1 AND status <> 'promoted'`, id, status)
	if err != nil {
		return DiscoveryResult{}, err
	}
	if tag.RowsAffected() == 0 {
		return DiscoveryResult{}, ErrConflict{Msg: "result not found or already promoted"}
	}
	if err := Audit(ctx, q, a, "discovery.result_"+status, "discovery_result", &id, nil, nil); err != nil {
		return DiscoveryResult{}, err
	}
	return scanResult(q.QueryRow(ctx, `SELECT `+resultCols+` FROM discovery_results r LEFT JOIN discovery_jobs j ON j.id = r.last_job_id WHERE r.id = $1`, id))
}

// osmSource returns (creating on first use) the data source for OpenStreetMap-derived records:
// internal use only, because ODbL attribution/share-alike terms need review before display or resale.
func osmSource(ctx context.Context, tx DBTX, a Actor) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `SELECT id FROM data_sources WHERE provider = 'openstreetmap' AND source_type = 'licensed_feed'
		ORDER BY created_at LIMIT 1`).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !IsNotFound(err) {
		return "", err
	}
	src, err := CreateSource(ctx, tx, a, SourceInput{SourceType: "licensed_feed", Provider: "openstreetmap",
		SourceURL: ptr("https://www.openstreetmap.org/copyright"), LicenseReference: ptr("ODbL-1.0 © OpenStreetMap contributors"),
		AllowedInternalUse: ptr(true),
		Notes:              ptr("Leads from OpenStreetMap via Overpass. Internal use only until ODbL attribution and share-alike obligations are reviewed.")})
	return src.ID, err
}

// PromoteResult turns an OpenStreetMap result into a draft company (internal-only source) and
// flags duplicates. Google results cannot be promoted because no content is stored for them.
func (s *Store) PromoteResult(ctx context.Context, a Actor, id string) (string, error) {
	var companyID string
	err := s.InTx(ctx, func(tx pgx.Tx) error {
		r, err := scanResult(tx.QueryRow(ctx, `SELECT `+resultCols+` FROM discovery_results r LEFT JOIN discovery_jobs j ON j.id = r.last_job_id
			WHERE r.id = $1 FOR UPDATE OF r`, id))
		if err != nil {
			return err
		}
		if r.Status == "promoted" {
			return ErrConflict{Msg: "result already promoted"}
		}
		if r.Provider != ProviderOSM || r.Name == nil || *r.Name == "" {
			return ErrConflict{Msg: "only named OpenStreetMap results can be promoted; for Google place IDs, contact the business and ask it to register"}
		}
		src, err := osmSource(ctx, tx, a)
		if err != nil {
			return err
		}
		in := CompanyInput{SourceID: &src, DisplayName: r.Name}
		if w := r.Tags["website"]; w != "" {
			in.WebsiteURL = &w
		} else if w := r.Tags["contact:website"]; w != "" {
			in.WebsiteURL = &w
		}
		if in.WebsiteURL != nil {
			if _, _, ok := normalize.WebsiteURL(*in.WebsiteURL); !ok {
				in.WebsiteURL = nil
			}
		}
		if r.CategorySlug != nil {
			var catID string
			if err := tx.QueryRow(ctx, `SELECT id FROM categories WHERE slug = $1`, *r.CategorySlug).Scan(&catID); err == nil {
				in.CategoryIDs = &[]string{catID}
				in.PrimaryCategoryID = &catID
			}
		}
		if companyID, err = createCompanyTx(ctx, tx, a, in, "draft"); err != nil {
			return err
		}
		city := r.Tags["addr:city"]
		if city == "" && r.JobLocation != nil {
			city = *r.JobLocation
		}
		fi := FacilityInput{SourceID: src, FacilityType: "factory", City: strOrNil(city), StateCode: r.JobStateCode,
			AddressLine1: strOrNil(r.Tags["addr:street"]), Locality: strOrNil(r.Tags["addr:suburb"])}
		if pc, ok := normalize.PostalCode(r.Tags["addr:postcode"]); ok {
			fi.PostalCode = &pc
		}
		if _, err := insertFacility(ctx, tx, companyID, fi); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE discovery_results SET status = 'promoted', matched_company_id = $2 WHERE id = $1`, id, companyID); err != nil {
			return err
		}
		if _, err := FindDuplicatesFor(ctx, tx, companyID); err != nil {
			return err
		}
		return Audit(ctx, tx, a, "discovery.result_promote", "discovery_result", &id, nil, map[string]any{"company_id": companyID})
	})
	return companyID, err
}
