package api_test

import (
	"context"
	"strings"
	"testing"
	"time"
)

const testWorkerToken = "test-worker-token-0123456789abcdef0123456789"

const jobsCSV = `job_id,priority,sector_family,category_slug,keyword,search_query,osm_name_regex,location,state_code,state_name,latitude,longitude,radius_km,location_tier,is_cluster,status
j-morbi-tiles,1,Building materials,vitrified-tiles,vitrified tiles,"vitrified tiles manufacturer in Morbi, Gujarat",vitrified|granito,Morbi,GJ,Gujarat,22.8173,70.8377,15,1,true,pending
j-pune-cnc,2,Metals and engineering,cnc-machined-components,CNC machining,"CNC machining job shop in Pune, Maharashtra",cnc|machining,Pune,MH,Maharashtra,18.5204,73.8567,25,1,false,pending
j-bad,2,X,nope-slug,kw,q,,Nowhere,ZZ,,999,0,5,3,false,pending
`

type worker struct {
	c  *client
	id string
}

func (e *env) worker(id string) worker {
	c := e.anon()
	return worker{c: c, id: id}
}

func (w worker) call(path string, body any, want int, out any) []byte {
	return w.c.must("POST", path, body, want, out, "Authorization", "Bearer "+testWorkerToken)
}

type claimed struct {
	Jobs []struct {
		ID           string  `json:"id"`
		JobKey       string  `json:"job_key"`
		Provider     string  `json:"provider"`
		OSMNameRegex *string `json:"osm_name_regex"`
		Attempts     int     `json:"attempts"`
	} `json:"jobs"`
}

func TestDiscoveryJobQueue(t *testing.T) {
	e := newEnv(t)
	admin := e.login("admin")
	var batch struct {
		Inserted, Updated, Skipped int
		Errors                     []string
	}
	admin.must("POST", "/api/v1/admin/discovery/jobs/upload", csvUpload("jobs.csv", jobsCSV, nil), 201, &batch)
	if batch.Inserted != 2 || batch.Skipped != 1 || len(batch.Errors) != 1 {
		t.Fatalf("upload = %+v", batch)
	}

	// Worker endpoints require the token.
	anon := e.anon()
	if code, _, _ := anon.do("POST", "/api/v1/worker/jobs/claim", map[string]any{"worker_id": "x"}); code != 401 {
		t.Fatalf("claim without token = %d", code)
	}

	w1, w2 := e.worker("cf-1"), e.worker("cf-2")
	var c1, c2 claimed
	w1.call("/api/v1/worker/jobs/claim", map[string]any{"worker_id": w1.id, "limit": 1}, 200, &c1)
	w2.call("/api/v1/worker/jobs/claim", map[string]any{"worker_id": w2.id, "limit": 4}, 200, &c2)
	if len(c1.Jobs) != 1 || c1.Jobs[0].JobKey != "j-morbi-tiles" || len(c2.Jobs) != 1 || c2.Jobs[0].JobKey != "j-pune-cnc" {
		t.Fatalf("claims must be disjoint and priority-ordered: %+v / %+v", c1, c2)
	}
	var none claimed
	w1.call("/api/v1/worker/jobs/claim", map[string]any{"worker_id": w1.id}, 200, &none)
	if len(none.Jobs) != 0 {
		t.Fatal("nothing should be due while jobs are leased")
	}

	// Wrong worker cannot complete someone else's job.
	w2.call("/api/v1/worker/jobs/"+c1.Jobs[0].ID+"/complete", map[string]any{"worker_id": w2.id, "results": []any{}}, 409, nil)

	lat, lng := 22.81, 70.83
	var done struct{ Received, New int }
	w1.call("/api/v1/worker/jobs/"+c1.Jobs[0].ID+"/complete", map[string]any{"worker_id": w1.id, "results": []map[string]any{
		{"provider_ref": "way/1001", "name": "Sample Vitrified Pvt Ltd", "latitude": lat, "longitude": lng,
			"tags": map[string]string{"man_made": "works", "addr:city": "Morbi", "phone": "+91 99999 00000", "website": "https://sample.example"}},
		{"provider_ref": "node/2002", "name": "Another Ceramic Works"},
	}}, 200, &done)
	if done.Received != 2 || done.New != 2 {
		t.Fatalf("complete = %+v", done)
	}
	var phoneKept int
	_ = e.st.Pool.QueryRow(context.Background(), `SELECT count(*) FROM discovery_results WHERE tags ? 'phone'`).Scan(&phoneKept)
	if phoneKept != 0 {
		t.Fatal("non-allowlisted tags (phone) must be dropped")
	}
	var next time.Time
	_ = e.st.Pool.QueryRow(context.Background(), `SELECT next_run_after FROM discovery_jobs WHERE job_key = 'j-morbi-tiles'`).Scan(&next)
	if time.Until(next) < 29*24*time.Hour {
		t.Fatalf("completed job should refresh in ~30 days, next=%v", next)
	}
	// Seen again on a later run: deduplicated.
	if _, err := e.st.Pool.Exec(context.Background(), `UPDATE discovery_jobs SET next_run_after = now() - interval '1 minute' WHERE job_key = 'j-morbi-tiles'`); err != nil {
		t.Fatal(err)
	}
	var again claimed
	w1.call("/api/v1/worker/jobs/claim", map[string]any{"worker_id": w1.id}, 200, &again)
	if len(again.Jobs) != 1 {
		t.Fatalf("due refresh job not reclaimed: %+v", again)
	}
	w1.call("/api/v1/worker/jobs/"+again.Jobs[0].ID+"/complete", map[string]any{"worker_id": w1.id, "results": []map[string]any{
		{"provider_ref": "way/1001", "name": "Sample Vitrified Pvt Ltd"}}}, 200, &done)
	if done.New != 0 {
		t.Fatal("re-seen result must not be counted as new")
	}

	// Failures back off, then fail after max_attempts; retry_failed requeues.
	job := c2.Jobs[0].ID
	for i := 0; i < 3; i++ {
		if i > 0 {
			_, _ = e.st.Pool.Exec(context.Background(), `UPDATE discovery_jobs SET next_run_after = now() - interval '1 minute' WHERE id = $1`, job)
			var c claimed
			w2.call("/api/v1/worker/jobs/claim", map[string]any{"worker_id": w2.id}, 200, &c)
			if len(c.Jobs) != 1 {
				t.Fatalf("attempt %d: job not reclaimed", i+1)
			}
		}
		var j struct{ Status string }
		w2.call("/api/v1/worker/jobs/"+job+"/fail", map[string]any{"worker_id": w2.id, "error": "overpass 429"}, 200, &j)
		want := "pending"
		if i == 2 {
			want = "failed"
		}
		if j.Status != want {
			t.Fatalf("after failure %d status = %s, want %s", i+1, j.Status, want)
		}
	}
	var affected struct{ Affected int }
	admin.must("POST", "/api/v1/admin/discovery/jobs/actions", map[string]any{"action": "retry_failed"}, 200, &affected)
	if affected.Affected != 1 {
		t.Fatalf("retry_failed affected %d", affected.Affected)
	}

	// Expired leases are reclaimable by another worker.
	var c3 claimed
	w1.call("/api/v1/worker/jobs/claim", map[string]any{"worker_id": w1.id}, 200, &c3)
	_, _ = e.st.Pool.Exec(context.Background(), `UPDATE discovery_jobs SET lease_expires_at = now() - interval '1 second' WHERE id = $1`, c3.Jobs[0].ID)
	var c4 claimed
	w2.call("/api/v1/worker/jobs/claim", map[string]any{"worker_id": w2.id}, 200, &c4)
	if len(c4.Jobs) != 1 || c4.Jobs[0].ID != c3.Jobs[0].ID {
		t.Fatal("expired lease should be reclaimable")
	}

	// Re-upload keeps run state; reset_state requeues.
	admin.must("POST", "/api/v1/admin/discovery/jobs/upload", csvUpload("jobs.csv", jobsCSV, nil), 201, &batch)
	if batch.Updated != 2 || batch.Inserted != 0 {
		t.Fatalf("re-upload = %+v", batch)
	}
	var status string
	_ = e.st.Pool.QueryRow(context.Background(), `SELECT status FROM discovery_jobs WHERE job_key = 'j-morbi-tiles'`).Scan(&status)
	if status != "done" {
		t.Fatalf("re-upload without reset must keep state, got %s", status)
	}
	admin.must("POST", "/api/v1/admin/discovery/jobs/upload", csvUpload("jobs.csv", jobsCSV, map[string]string{"reset_state": "true"}), 201, &batch)
	_ = e.st.Pool.QueryRow(context.Background(), `SELECT status FROM discovery_jobs WHERE job_key = 'j-morbi-tiles'`).Scan(&status)
	if status != "pending" {
		t.Fatalf("reset_state should requeue, got %s", status)
	}

	var stats struct {
		ByStatus     map[string]int `json:"by_status"`
		TotalResults int            `json:"total_results"`
	}
	admin.must("GET", "/api/v1/admin/discovery/jobs/stats", nil, 200, &stats)
	if stats.TotalResults != 2 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestDiscoveryResultsPromotionAndGoogleBoundary(t *testing.T) {
	e := newEnv(t)
	admin := e.login("admin")
	csv := jobsCSV + "j-google,1,,tiles,tiles,\"tiles in Morbi\",,Morbi,GJ,Gujarat,22.8,70.8,10,1,false,pending\n"
	csv = strings.Replace(csv, "job_id,priority", "job_id,priority", 1)
	// add a provider column for the google row
	lines := strings.Split(strings.TrimSpace(csv), "\n")
	lines[0] += ",provider"
	for i := 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "j-google") {
			lines[i] += ",google_places_ids"
		} else {
			lines[i] += ",osm_overpass"
		}
	}
	admin.must("POST", "/api/v1/admin/discovery/jobs/upload", csvUpload("jobs.csv", strings.Join(lines, "\n")+"\n", nil), 201, nil)

	w := e.worker("cf-1")
	var c claimed
	w.call("/api/v1/worker/jobs/claim", map[string]any{"worker_id": w.id, "limit": 5, "providers": []string{"osm_overpass", "google_places_ids"}}, 200, &c)
	for _, j := range c.Jobs {
		results := []map[string]any{{"provider_ref": "ref-" + j.JobKey, "name": "Morvi Vitrified Tiles", "tags": map[string]string{"addr:postcode": "363641"}}}
		w.call("/api/v1/worker/jobs/"+j.ID+"/complete", map[string]any{"worker_id": w.id, "results": results}, 200, nil)
	}
	var googleNamed int
	_ = e.st.Pool.QueryRow(context.Background(), `SELECT count(*) FROM discovery_results WHERE provider = 'google_places_ids' AND name IS NOT NULL`).Scan(&googleNamed)
	if googleNamed != 0 {
		t.Fatal("Google results must store the place ID only")
	}
	if _, err := e.st.Pool.Exec(context.Background(), `INSERT INTO discovery_results (provider, provider_ref, name) VALUES ('google_places_ids', 'x', 'Leak')`); err == nil {
		t.Fatal("database must reject Google content")
	}

	var results struct {
		Data []struct {
			ID, Provider string
		} `json:"data"`
	}
	admin.must("GET", "/api/v1/admin/discovery/results?limit=50", nil, 200, &results)
	var osmID, googleID string
	for _, r := range results.Data {
		if r.Provider == "google_places_ids" {
			googleID = r.ID
		} else if osmID == "" {
			osmID = r.ID
		}
	}
	admin.must("POST", "/api/v1/admin/discovery/results/"+googleID+"/promote", nil, 409, nil)
	var promoted struct {
		CompanyID string `json:"company_id"`
	}
	admin.must("POST", "/api/v1/admin/discovery/results/"+osmID+"/promote", nil, 200, &promoted)
	admin.must("POST", "/api/v1/admin/discovery/results/"+osmID+"/promote", nil, 409, nil)

	var company struct {
		PublicationStatus string `json:"publication_status"`
		Facilities        []struct {
			City       *string `json:"city"`
			PostalCode *string `json:"postal_code"`
		} `json:"facilities"`
		PublishCheck struct{ Allowed bool } `json:"publish_check"`
	}
	admin.must("GET", "/api/v1/admin/companies/"+promoted.CompanyID, nil, 200, &company)
	if company.PublicationStatus != "draft" || company.PublishCheck.Allowed || len(company.Facilities) != 1 ||
		company.Facilities[0].PostalCode == nil || *company.Facilities[0].PostalCode != "363641" {
		t.Fatalf("promoted company = %+v", company)
	}
	var dups struct{ Data []any }
	admin.must("GET", "/api/v1/admin/duplicates", nil, 200, &dups)
	if len(dups.Data) == 0 {
		t.Fatal("promotion should flag the synthetic Morvi Vitrified record as a possible duplicate")
	}
	admin.must("PATCH", "/api/v1/admin/discovery/results/"+googleID, map[string]string{"status": "contacted"}, 200, nil)
}

const testInternalToken = "test-internal-token-0123456789abcdef01234567"

func TestPublicRateLimitAndInternalBypass(t *testing.T) {
	e := newEnv(t)
	c := e.anon()
	limited := false
	for i := 0; i < 70; i++ { // burst is 60
		if code, _, _ := c.do("GET", "/api/v1/roles", nil); code == 429 {
			limited = true
			break
		}
	}
	if !limited {
		t.Fatal("anonymous public requests should hit the per-IP limit")
	}
	for i := 0; i < 20; i++ {
		if code, _, _ := c.do("GET", "/api/v1/roles", nil, "X-Internal-Token", testInternalToken); code != 200 {
			t.Fatalf("internal web-server request %d got %d", i, code)
		}
	}
	if code, _, _ := c.do("GET", "/api/v1/roles", nil, "X-Internal-Token", "wrong-token-wrong-token-wrong-token"); code != 429 {
		t.Fatalf("wrong internal token must not bypass the limit, got %d", code)
	}
}

func TestGmapsLeadsReviewAndPromote(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	pool := e.st.Pool
	var jobID string
	if err := pool.QueryRow(ctx, `INSERT INTO discovery_jobs (job_key, provider, keyword, search_query, category_slug, location, state_code, latitude, longitude, radius_km)
		VALUES ('gm1', 'gmaps_scraper', 'tiles', 'tiles manufacturer in Morbi, Gujarat', (SELECT slug FROM categories ORDER BY slug LIMIT 1), 'Morbi', 'GJ', 22.8, 70.8, 15)
		RETURNING id`).Scan(&jobID); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO gmaps_leads (place_key, first_job_id, last_job_id, category_slug, state_code, location, title, address, phone, website, complete_address)
		 SELECT 'p1', $1, $1, category_slug, 'GJ', 'Morbi', 'Acme Tiles', '8-A Highway, Morbi, Gujarat 363642', '+91 98250 12345', 'https://acmetiles.example', '{"city":"Morbi","postal_code":"363642"}'
		 FROM discovery_jobs WHERE id = $1`,
		`INSERT INTO gmaps_leads (place_key, first_job_id, last_job_id, state_code, location, title) VALUES ('p2', $1, $1, 'GJ', 'Morbi', 'Beta Ceramics')`,
	} {
		if _, err := pool.Exec(ctx, q, jobID); err != nil {
			t.Fatal(err)
		}
	}

	reviewer := e.login("reviewer")
	var page struct {
		Data []struct {
			ID                string  `json:"id"`
			Title             string  `json:"title"`
			Status            string  `json:"status"`
			SearchQuery       *string `json:"search_query"`
			PromotedCompanyID *string `json:"promoted_company_id"`
		} `json:"data"`
		Page struct {
			Total int `json:"total"`
		} `json:"page"`
	}
	reviewer.must("GET", "/api/v1/admin/gmaps/leads?job=gm1", nil, 200, &page)
	if page.Page.Total != 2 || page.Data[0].SearchQuery == nil {
		t.Fatalf("list = %+v", page)
	}
	reviewer.must("GET", "/api/v1/admin/gmaps/leads?has_phone=true&q=acme", nil, 200, &page)
	if page.Page.Total != 1 || page.Data[0].Title != "Acme Tiles" {
		t.Fatalf("filtered list = %+v", page)
	}
	acme := page.Data[0].ID
	reviewer.must("POST", "/api/v1/admin/gmaps/leads/"+acme+"/promote", nil, 403, nil)

	admin := e.login("admin")
	var promoted struct {
		CompanyID string `json:"company_id"`
	}
	admin.must("POST", "/api/v1/admin/gmaps/leads/"+acme+"/promote", nil, 200, &promoted)
	admin.must("POST", "/api/v1/admin/gmaps/leads/"+acme+"/promote", nil, 409, nil)
	admin.must("PATCH", "/api/v1/admin/gmaps/leads/"+acme, map[string]string{"status": "dismissed"}, 409, nil)

	var status, city, postal, srcType string
	var public bool
	var phones int
	if err := pool.QueryRow(ctx, `SELECT c.publication_status, f.city, f.postal_code, ds.source_type, ds.allowed_public_display,
		(SELECT count(*) FROM contacts ct WHERE ct.company_id = c.id AND ct.channel = 'phone' AND ct.value IS NULL AND ct.value_encrypted IS NOT NULL)
		FROM companies c JOIN facilities f ON f.company_id = c.id JOIN field_provenance fp ON fp.company_id = c.id AND fp.field_name = 'display_name'
		JOIN data_sources ds ON ds.id = fp.source_id WHERE c.id = $1`, promoted.CompanyID).Scan(&status, &city, &postal, &srcType, &public, &phones); err != nil {
		t.Fatal(err)
	}
	if status != "draft" || city != "Morbi" || postal != "363642" || srcType != "public_web_listing" || public || phones != 1 {
		t.Fatalf("promoted company: status=%s city=%s postal=%s source=%s public=%v phones=%d", status, city, postal, srcType, public, phones)
	}

	reviewer.must("GET", "/api/v1/admin/gmaps/leads?q=beta", nil, 200, &page)
	beta := page.Data[0].ID
	reviewer.must("PATCH", "/api/v1/admin/gmaps/leads/"+beta, map[string]string{"status": "contacted"}, 200, nil)
	reviewer.must("GET", "/api/v1/admin/gmaps/leads?status=promoted", nil, 200, &page)
	if page.Page.Total != 1 || page.Data[0].PromotedCompanyID == nil || *page.Data[0].PromotedCompanyID != promoted.CompanyID {
		t.Fatalf("promoted filter = %+v", page)
	}
}

func TestGmapsLeadsBulkPromoteAndApprove(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	pool := e.st.Pool
	if _, err := pool.Exec(ctx, `
		WITH j AS (INSERT INTO discovery_jobs (job_key, provider, keyword, search_query, location, state_code, latitude, longitude, radius_km)
		           VALUES ('gmb', 'gmaps_scraper', 'k', 'q', 'Morbi', 'GJ', 22.8, 70.8, 15) RETURNING id)
		INSERT INTO gmaps_leads (place_key, first_job_id, last_job_id, state_code, location, title, phone, status)
		SELECT 'b' || g, j.id, j.id, CASE WHEN g <= 4 THEN 'GJ' ELSE 'MH' END, 'Morbi', 'Bulk Lead ' || g,
		       CASE WHEN g <> 2 THEN '+91 98250 1000' || g END, CASE WHEN g = 3 THEN 'dismissed' ELSE 'new' END
		FROM j, generate_series(1, 6) g`); err != nil {
		t.Fatal(err)
	}
	e.login("reviewer").must("POST", "/api/v1/admin/gmaps/leads/promote", map[string]any{}, 403, nil)
	admin := e.login("admin")

	// Gujarat + has phone: leads 1 and 4 (2 has no phone, 3 is dismissed, 5-6 are MH).
	var res struct {
		Promoted, Failed, Remaining int
	}
	admin.must("POST", "/api/v1/admin/gmaps/leads/promote", map[string]any{"state": "GJ", "has_phone": true}, 200, &res)
	if res.Promoted != 2 || res.Failed != 0 || res.Remaining != 0 {
		t.Fatalf("bulk promote = %+v, want 2 promoted, 0 remaining", res)
	}
	admin.must("POST", "/api/v1/admin/gmaps/leads/promote", map[string]any{"state": "GJ", "has_phone": true}, 200, &res)
	if res.Promoted != 0 {
		t.Fatalf("second run promoted again: %+v", res)
	}

	var approved struct{ Approved int }
	admin.must("POST", "/api/v1/admin/gmaps/leads/approve-promoted", nil, 200, &approved)
	if approved.Approved != 2 {
		t.Fatalf("approved = %d, want 2", approved.Approved)
	}
	var drafts, approvedPrivate, dismissedStill int
	_ = pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE c.publication_status = 'draft'), count(*) FILTER (WHERE c.publication_status = 'approved_private')
		FROM companies c JOIN gmaps_leads l ON l.promoted_company_id = c.id`).Scan(&drafts, &approvedPrivate)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM gmaps_leads WHERE place_key = 'b3' AND status = 'dismissed'`).Scan(&dismissedStill)
	if drafts != 0 || approvedPrivate != 2 || dismissedStill != 1 {
		t.Fatalf("drafts=%d approved=%d dismissed=%d", drafts, approvedPrivate, dismissedStill)
	}
	admin.must("POST", "/api/v1/admin/gmaps/leads/approve-promoted", nil, 200, &approved)
	if approved.Approved != 0 {
		t.Fatalf("second approve = %d", approved.Approved)
	}
}
