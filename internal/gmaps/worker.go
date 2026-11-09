package gmaps

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/bharatmfg/directory/internal/store"
)

// Config controls the worker. All values come from env vars (see ConfigFromEnv).
type Config struct {
	APIURL   string        // GMAPS_API_URL
	Auth     string        // GMAPS_AUTH "user:password"
	WorkerID string        // GMAPS_WORKER_ID (default: hostname)
	Depth    int           // GMAPS_DEPTH 1-10
	Email    bool          // GMAPS_EMAIL
	MaxTime  time.Duration // GMAPS_MAX_TIME_SECONDS, per scraper job
	Idle     time.Duration // GMAPS_IDLE_SECONDS, pause when no job is due
	Poll     time.Duration // GMAPS_POLL_SECONDS, how often to check the scraper
}

// ConfigFromEnv reads the worker settings with sensible defaults.
func ConfigFromEnv() Config {
	host, _ := os.Hostname()
	c := Config{
		APIURL:   envStr("GMAPS_API_URL", "https://leads.peoplvance.org/api/v1/jobs"),
		Auth:     os.Getenv("GMAPS_AUTH"),
		WorkerID: envStr("GMAPS_WORKER_ID", "gmaps-"+host),
		Depth:    envInt("GMAPS_DEPTH", 5),
		Email:    os.Getenv("GMAPS_EMAIL") == "true" || os.Getenv("GMAPS_EMAIL") == "1",
		Idle:     time.Duration(envInt("GMAPS_IDLE_SECONDS", 300)) * time.Second,
		Poll:     time.Duration(envInt("GMAPS_POLL_SECONDS", 20)) * time.Second,
	}
	defMax := 300
	if c.Email {
		defMax = 900
	}
	c.MaxTime = time.Duration(max(envInt("GMAPS_MAX_TIME_SECONDS", defMax), 180)) * time.Second
	c.Depth = min(max(c.Depth, 1), 10)
	return c
}

func envStr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(k)); err == nil && n > 0 {
		return n
	}
	return def
}

// Worker runs gmaps_scraper discovery jobs one at a time.
type Worker struct {
	Cfg    Config
	Client *Client
	Store  *store.Store
	Log    *slog.Logger
}

// NewWorker builds a worker with an HTTP client for the scraper.
func NewWorker(cfg Config, st *store.Store, log *slog.Logger) *Worker {
	return &Worker{Cfg: cfg, Store: st, Log: log,
		Client: &Client{BaseURL: cfg.APIURL, Auth: cfg.Auth, HTTP: &http.Client{Timeout: 2 * time.Minute}}}
}

// Run loops until ctx is cancelled: claim one due job → scrape → save → repeat.
// When the queue is empty it pauses for Cfg.Idle and checks again.
func (w *Worker) Run(ctx context.Context) error {
	w.Log.Info("gmaps worker started", "worker_id", w.Cfg.WorkerID, "scraper", w.Cfg.APIURL, "depth", w.Cfg.Depth,
		"email", w.Cfg.Email, "idle", w.Cfg.Idle.String())
	idle := false
	for ctx.Err() == nil {
		jobs, err := store.ClaimJobs(ctx, w.Store.Pool, w.Cfg.WorkerID, 1, []string{store.ProviderGmaps})
		if err != nil {
			if ctx.Err() == nil {
				w.Log.Error("claim failed, retrying in 1 minute", "err", err)
			}
			sleep(ctx, time.Minute)
			continue
		}
		if len(jobs) == 0 {
			if !idle {
				w.Log.Info("no jobs due, pausing", "check_every", w.Cfg.Idle.String())
				idle = true
			}
			sleep(ctx, w.Cfg.Idle)
			continue
		}
		idle = false
		w.RunJob(ctx, jobs[0])
	}
	w.Log.Info("gmaps worker stopped")
	return nil
}

// RunJob scrapes one claimed job and records the outcome on it.
func (w *Worker) RunJob(ctx context.Context, j store.DiscoveryJob) {
	log := w.Log.With("job_key", j.JobKey, "query", j.SearchQuery)
	log.Info("job started", "attempt", j.Attempts)
	start := time.Now()
	leads, err := w.scrape(ctx, j)
	// Use a fresh context so the outcome is recorded even while shutting down.
	bg, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	switch {
	case ctx.Err() != nil:
		if err := store.ReleaseJob(bg, w.Store.Pool, j.ID, w.Cfg.WorkerID); err != nil {
			log.Error("release failed", "err", err)
		}
		log.Info("shutting down, job handed back")
	case err != nil:
		fj, ferr := store.FailJob(bg, w.Store.Pool, j.ID, w.Cfg.WorkerID, err.Error())
		if ferr != nil {
			log.Error("could not record failure", "err", ferr, "scrape_err", err)
			return
		}
		log.Warn("job failed", "err", err, "status", fj.Status, "retry_after", fj.NextRunAfter)
	default:
		res, err := w.Store.CompleteGmapsJob(bg, j.ID, w.Cfg.WorkerID, leads)
		if err != nil {
			log.Error("saving leads failed", "err", err)
			if _, ferr := store.FailJob(bg, w.Store.Pool, j.ID, w.Cfg.WorkerID, "save: "+err.Error()); ferr != nil {
				log.Error("could not record failure", "err", ferr)
			}
			return
		}
		log.Info("job done", "leads", res.Received, "new", res.New, "took", time.Since(start).Round(time.Second).String())
	}
}

// scrape sends one search to the scraper, waits for it, and returns the parsed leads.
func (w *Worker) scrape(ctx context.Context, j store.DiscoveryJob) ([]store.GmapsLead, error) {
	id, err := w.Client.Create(ctx, JobRequest{Name: "suppliers-" + j.JobKey, Keywords: []string{j.SearchQuery}, Lang: "en",
		Depth: w.Cfg.Depth, MaxTime: int(w.Cfg.MaxTime.Seconds()), Email: w.Cfg.Email})
	if err != nil {
		return nil, err
	}
	defer func() { // clean up the scraper server's disk whatever happens
		bg, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := w.Client.Delete(bg, id); err != nil {
			w.Log.Warn("could not delete scraper job", "scraper_job", id, "err", err)
		}
	}()

	// The scraper runs jobs one at a time, so allow queue time on top of the job's own max time.
	deadline := time.Now().Add(w.Cfg.MaxTime + 30*time.Minute)
	for {
		if !sleep(ctx, w.Cfg.Poll) {
			return nil, ctx.Err()
		}
		if err := store.ExtendLease(ctx, w.Store.Pool, j.ID, w.Cfg.WorkerID); err != nil {
			return nil, fmt.Errorf("lost lease: %w", err)
		}
		status, err := w.Client.Status(ctx, id)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			w.Log.Warn("status check failed, will retry", "scraper_job", id, "err", err)
		}
		switch status {
		case "ok":
			data, err := w.Client.Download(ctx, id)
			if err != nil {
				return nil, err
			}
			if len(data) == 0 {
				return nil, errors.New("scraper returned an empty CSV (blocked by Google or no results; check proxies)")
			}
			return ParseCSV(data)
		case "failed":
			return nil, errors.New("scraper job failed")
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("scraper job still %q after %s", status, w.Cfg.MaxTime+30*time.Minute)
		}
	}
}

// sleep waits d or until ctx is done; it reports whether the full wait happened.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
