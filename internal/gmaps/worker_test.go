package gmaps_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bharatmfg/directory/internal/gmaps"
	"github.com/bharatmfg/directory/internal/secure"
	"github.com/bharatmfg/directory/internal/store"
	"github.com/bharatmfg/directory/internal/testutil"
)

const resultsCSV = "input_id,link,title,category,address,website,phone,place_id,cid,latitude,longitude,review_count,review_rating,status,complete_address\n" +
	`x,https://maps/a,Acme Tiles,Tile manufacturer,"Morbi, Gujarat",https://acme.in,+91 99999 00000,ChIJ-acme,1,22.81,70.83,12,4.5,OPERATIONAL,"{""city"":""Morbi""}"` + "\n" +
	`x,https://maps/a,Acme Tiles dup,,,,,ChIJ-acme,,,,,,,` + "\n" +
	`x,https://maps/b,Beta Ceramics,,,,,,2,,,,,,` + "\n" +
	`x,https://maps/c,,,,,,ChIJ-noname,,,,,,,` + "\n"

func TestParseCSV(t *testing.T) {
	leads, err := gmaps.ParseCSV([]byte(resultsCSV))
	if err != nil {
		t.Fatal(err)
	}
	if len(leads) != 2 {
		t.Fatalf("leads = %d, want 2 (dup and nameless skipped)", len(leads))
	}
	if leads[0].PlaceKey != "ChIJ-acme" || *leads[0].ReviewCount != 12 || string(leads[0].CompleteAddress) != `{"city":"Morbi"}` {
		t.Fatalf("lead 0 = %+v", leads[0])
	}
	if leads[1].PlaceKey != "2" || leads[1].Latitude != nil {
		t.Fatalf("lead 1 = %+v (cid fallback, empty lat)", leads[1])
	}
	if l, err := gmaps.ParseCSV(nil); err != nil || l != nil {
		t.Fatalf("empty = %v, %v", l, err)
	}
}

// fakeScraper mimics the scraper API: jobs finish on the second status check.
type fakeScraper struct {
	mu           sync.Mutex
	checks       map[string]int
	created      []gmaps.JobRequest
	deleted      []string
	csv          string
	stuck        bool   // never reports "ok", like the real scraper when it hangs
	blockedUntil string // file: jobs stay "pending" until it exists (another job blocks the queue)
}

func (f *fakeScraper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/jobs")
	switch {
	case r.Method == http.MethodPost && path == "":
		var in gmaps.JobRequest
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &in)
		f.created = append(f.created, in)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"11111111-1111-1111-1111-111111111111"}`))
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/download"):
		_, _ = w.Write([]byte(f.csv))
	case r.Method == http.MethodGet:
		if f.blockedUntil != "" {
			if _, err := os.Stat(f.blockedUntil); err != nil {
				_, _ = w.Write([]byte(`{"ID":"x","Status":"pending"}`))
				return
			}
		}
		f.checks[path]++
		st := "working"
		if f.checks[path] >= 2 && !f.stuck {
			st = "ok"
		}
		_, _ = w.Write([]byte(`{"ID":"x","Status":"` + st + `"}`))
	case r.Method == http.MethodDelete:
		f.deleted = append(f.deleted, path)
	}
}

func TestWorkerRunsOneJobAndSavesLeads(t *testing.T) {
	pool := testutil.NewDB(t)
	testutil.Seed(t, pool)
	box, err := secure.NewBox("dGVzdC1rZXktdGVzdC1rZXktdGVzdC1rZXktdGVzdC1rZXk=")
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(pool, box)
	ctx := context.Background()
	for _, q := range []string{
		`INSERT INTO discovery_jobs (job_key, provider, keyword, search_query, location, state_code, latitude, longitude, radius_km)
		 VALUES ('g1', 'gmaps_scraper', 'tiles', 'tiles manufacturer in Morbi, Gujarat', 'Morbi', 'GJ', 22.8, 70.8, 15)`,
		`INSERT INTO discovery_jobs (job_key, provider, keyword, search_query, location, latitude, longitude, radius_km)
		 VALUES ('osm1', 'osm_overpass', 'tiles', 'tiles in Morbi', 'Morbi', 22.8, 70.8, 15)`,
	} {
		if _, err := pool.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}

	fake := &fakeScraper{checks: map[string]int{}, csv: resultsCSV}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	cfg := gmaps.Config{APIURL: srv.URL + "/api/v1/jobs", WorkerID: "w1", Depth: 5, MaxTime: 180 * time.Second,
		Idle: 50 * time.Millisecond, Poll: 10 * time.Millisecond}
	w := gmaps.NewWorker(cfg, st, slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Run until the gmaps job is done, then stop the loop (it would otherwise pause and poll forever).
	runCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	done := make(chan error)
	go func() { done <- w.Run(runCtx) }()
	for {
		var status string
		if err := pool.QueryRow(ctx, `SELECT status FROM discovery_jobs WHERE job_key = 'g1'`).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status == "done" || runCtx.Err() != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond) // let it go idle
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	var status, osmStatus string
	var results, leads, seen int
	var nextRun *time.Time
	_ = pool.QueryRow(ctx, `SELECT status, results_count, next_run_after FROM discovery_jobs WHERE job_key = 'g1'`).Scan(&status, &results, &nextRun)
	_ = pool.QueryRow(ctx, `SELECT status FROM discovery_jobs WHERE job_key = 'osm1'`).Scan(&osmStatus)
	_ = pool.QueryRow(ctx, `SELECT count(*), max(seen_count) FROM gmaps_leads WHERE state_code = 'GJ' AND location = 'Morbi'`).Scan(&leads, &seen)
	if status != "done" || results != 2 || nextRun == nil || nextRun.Before(time.Now().Add(29*24*time.Hour)) {
		t.Fatalf("gmaps job = %s results=%d next=%v", status, results, nextRun)
	}
	if osmStatus != "pending" {
		t.Fatalf("osm job touched: %s", osmStatus)
	}
	if leads != 2 || seen != 1 {
		t.Fatalf("leads = %d seen = %d", leads, seen)
	}
	if len(fake.created) != 1 || fake.created[0].Keywords[0] != "tiles manufacturer in Morbi, Gujarat" || fake.created[0].Depth != 5 {
		t.Fatalf("scraper calls = %+v", fake.created)
	}
	if len(fake.deleted) != 1 {
		t.Fatalf("scraper job not cleaned up: %v", fake.deleted)
	}
}

func TestWorkerRecordsEmptyResultAsFailure(t *testing.T) {
	pool := testutil.NewDB(t)
	testutil.Seed(t, pool)
	box, _ := secure.NewBox("dGVzdC1rZXktdGVzdC1rZXktdGVzdC1rZXktdGVzdC1rZXk=")
	st := store.New(pool, box)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO discovery_jobs (job_key, provider, keyword, search_query, location, latitude, longitude, radius_km)
		VALUES ('g2', 'gmaps_scraper', 'k', 'q', 'Pune', 18.5, 73.8, 10)`); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(&fakeScraper{checks: map[string]int{}, csv: ""})
	defer srv.Close()
	w := gmaps.NewWorker(gmaps.Config{APIURL: srv.URL + "/api/v1/jobs", WorkerID: "w1", Depth: 1, MaxTime: 180 * time.Second,
		Poll: 10 * time.Millisecond}, st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	jobs, err := store.ClaimJobs(ctx, pool, "w1", 1, []string{store.ProviderGmaps})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("claim = %v, %v", jobs, err)
	}
	w.RunJob(ctx, jobs[0])
	var status, lastErr string
	var next time.Time
	_ = pool.QueryRow(ctx, `SELECT status, last_error, next_run_after FROM discovery_jobs WHERE job_key = 'g2'`).Scan(&status, &lastErr, &next)
	if status != "pending" || !strings.Contains(lastErr, "empty CSV") || !next.After(time.Now()) {
		t.Fatalf("after empty result: %s %q %v", status, lastErr, next)
	}
}

func TestWorkerUsesResultsOfHungScraperJob(t *testing.T) {
	pool := testutil.NewDB(t)
	testutil.Seed(t, pool)
	box, _ := secure.NewBox("dGVzdC1rZXktdGVzdC1rZXktdGVzdC1rZXktdGVzdC1rZXk=")
	st := store.New(pool, box)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO discovery_jobs (job_key, provider, keyword, search_query, location, state_code, latitude, longitude, radius_km)
		VALUES ('g3', 'gmaps_scraper', 'tiles', 'tiles in Morbi', 'Morbi', 'GJ', 22.8, 70.8, 15)`); err != nil {
		t.Fatal(err)
	}
	fake := &fakeScraper{checks: map[string]int{}, csv: resultsCSV, stuck: true}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	marker := t.TempDir() + "/restarted"
	w := gmaps.NewWorker(gmaps.Config{APIURL: srv.URL + "/api/v1/jobs", WorkerID: "w1", Depth: 1,
		MaxTime: 20 * time.Millisecond, Poll: 10 * time.Millisecond, RestartCmd: "touch " + marker},
		st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	jobs, err := store.ClaimJobs(ctx, pool, "w1", 1, []string{store.ProviderGmaps})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("claim = %v, %v", jobs, err)
	}
	w.RunJob(ctx, jobs[0])

	var status string
	var results int
	_ = pool.QueryRow(ctx, `SELECT status, results_count FROM discovery_jobs WHERE job_key = 'g3'`).Scan(&status, &results)
	if status != "done" || results != 2 {
		t.Fatalf("hung job = %s results=%d, want done with 2 (results taken from the CSV)", status, results)
	}
	if len(fake.deleted) != 1 {
		t.Fatalf("scraper job not cleaned up: %v", fake.deleted)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("restart command did not run after the hung job")
	}
}

func TestWorkerRestartsScraperBlockedByAnotherJob(t *testing.T) {
	pool := testutil.NewDB(t)
	testutil.Seed(t, pool)
	box, _ := secure.NewBox("dGVzdC1rZXktdGVzdC1rZXktdGVzdC1rZXktdGVzdC1rZXk=")
	st := store.New(pool, box)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO discovery_jobs (job_key, provider, keyword, search_query, location, state_code, latitude, longitude, radius_km)
		VALUES ('g4', 'gmaps_scraper', 'tiles', 'tiles in Morbi', 'Morbi', 'GJ', 22.8, 70.8, 15)`); err != nil {
		t.Fatal(err)
	}
	// The scraper is busy with someone else's hung job: ours stays "pending" until the scraper restarts.
	restarted := t.TempDir() + "/restarted"
	fake := &fakeScraper{checks: map[string]int{}, csv: resultsCSV, blockedUntil: restarted}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	w := gmaps.NewWorker(gmaps.Config{APIURL: srv.URL + "/api/v1/jobs", WorkerID: "w1", Depth: 1,
		MaxTime: 20 * time.Millisecond, Poll: 10 * time.Millisecond, RestartCmd: "touch " + restarted},
		st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	jobs, err := store.ClaimJobs(ctx, pool, "w1", 1, []string{store.ProviderGmaps})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("claim = %v, %v", jobs, err)
	}
	w.RunJob(ctx, jobs[0])

	var status string
	var results int
	_ = pool.QueryRow(ctx, `SELECT status, results_count FROM discovery_jobs WHERE job_key = 'g4'`).Scan(&status, &results)
	if status != "done" || results != 2 {
		t.Fatalf("blocked job = %s results=%d, want done with 2 after the scraper restart", status, results)
	}
}
