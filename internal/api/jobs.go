package api

import (
	"crypto/subtle"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bharatmfg/directory/internal/httpx"
	"github.com/bharatmfg/directory/internal/store"
)

// --- admin: job queue ---

func (s *Server) uploadJobs(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		return httpx.E(http.StatusBadRequest, "invalid_upload", "expected multipart/form-data with a file field (max 64 MB)")
	}
	defer r.MultipartForm.RemoveAll() //nolint:errcheck
	file, hdr, err := r.FormFile("file")
	if err != nil {
		return httpx.FieldErrors(map[string]string{"file": "is required"})
	}
	defer file.Close()
	if ext := strings.ToLower(filepath.Ext(hdr.Filename)); ext != ".csv" {
		return httpx.FieldErrors(map[string]string{"file": "must be a .csv file"})
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	reset := r.FormValue("reset_state") == "true"
	b, err := s.Store.UploadJobs(r.Context(), s.actor(r), filepath.Base(hdr.Filename), data, reset)
	if err != nil {
		return translate(err)
	}
	httpx.JSON(w, http.StatusCreated, b)
	return nil
}

func (s *Server) listJobs(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	prio, _ := strconv.Atoi(q.Get("priority"))
	p := httpx.ParsePagination(r, 50)
	data, total, err := store.ListJobs(r.Context(), s.Store.Pool, store.JobFilter{Status: q.Get("status"), Provider: q.Get("provider"),
		StateCode: q.Get("state"), CategorySlug: q.Get("category"), Priority: prio, Q: q.Get("q")}, p.Limit, p.Offset())
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, httpx.NewPaged(data, p, total))
	return nil
}

func (s *Server) jobStats(w http.ResponseWriter, r *http.Request) error {
	st, err := store.GetJobStats(r.Context(), s.Store.Pool)
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, st)
	return nil
}

func (s *Server) jobAction(w http.ResponseWriter, r *http.Request) error {
	var in store.JobAction
	if err := httpx.Decode(w, r, &in, 64<<10); err != nil {
		return err
	}
	n, err := store.ApplyJobAction(r.Context(), s.Store.Pool, s.actor(r), in)
	if err != nil {
		return translate(err)
	}
	httpx.JSON(w, 200, map[string]int64{"affected": n})
	return nil
}

func (s *Server) listResults(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	p := httpx.ParsePagination(r, 50)
	data, total, err := store.ListResults(r.Context(), s.Store.Pool, q.Get("status"), q.Get("provider"), q.Get("category"), q.Get("q"), p.Limit, p.Offset())
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, httpx.NewPaged(data, p, total))
	return nil
}

type resultStatusBody struct {
	Status string `json:"status" validate:"required,oneof=new contacted dismissed"`
}

func (s *Server) updateResult(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in resultStatusBody
	if err := httpx.Decode(w, r, &in, 4<<10); err != nil {
		return err
	}
	res, err := store.SetResultStatus(r.Context(), s.Store.Pool, s.actor(r), id, in.Status)
	if err != nil {
		return translate(err)
	}
	httpx.JSON(w, 200, res)
	return nil
}

func (s *Server) promoteResult(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	companyID, err := s.Store.PromoteResult(r.Context(), s.actor(r), id)
	if err != nil {
		return translate(err)
	}
	httpx.JSON(w, 200, map[string]string{"company_id": companyID})
	return nil
}

// --- Google Maps leads (gmaps_leads, found by the gmaps worker) ---

func (s *Server) listGmapsLeads(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	p := httpx.ParsePagination(r, 50)
	f := store.GmapsLeadFilter{Status: q.Get("status"), Category: q.Get("category"), State: q.Get("state"),
		Search: strings.TrimSpace(q.Get("q")), JobKey: q.Get("job"),
		HasPhone: q.Get("has_phone") == "true", HasWebsite: q.Get("has_website") == "true"}
	data, total, err := store.ListGmapsLeads(r.Context(), s.Store.Pool, f, p.Limit, p.Offset())
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, httpx.NewPaged(data, p, total))
	return nil
}

func (s *Server) updateGmapsLead(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in resultStatusBody
	if err := httpx.Decode(w, r, &in, 4<<10); err != nil {
		return err
	}
	lead, err := store.SetGmapsLeadStatus(r.Context(), s.Store.Pool, s.actor(r), id, in.Status)
	if err != nil {
		return translate(err)
	}
	httpx.JSON(w, 200, lead)
	return nil
}

func (s *Server) promoteGmapsLead(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	companyID, err := s.Store.PromoteGmapsLead(r.Context(), s.actor(r), id)
	if err != nil {
		return translate(err)
	}
	httpx.JSON(w, 200, map[string]string{"company_id": companyID})
	return nil
}

// --- worker API (machine-to-machine, Bearer WORKER_TOKEN) ---

func (s *Server) requireWorker(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Cfg.WorkerToken == "" {
			httpx.WriteError(w, r, httpx.E(http.StatusServiceUnavailable, "workers_disabled", "set WORKER_TOKEN to enable job workers"))
			return
		}
		tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(tok), []byte(s.Cfg.WorkerToken)) != 1 {
			httpx.WriteError(w, r, httpx.E(http.StatusUnauthorized, "unauthorized", "invalid worker token"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

type claimBody struct {
	WorkerID  string   `json:"worker_id" validate:"required,max=100"`
	Limit     int      `json:"limit" validate:"omitempty,gte=1,lte=20"`
	Providers []string `json:"providers" validate:"omitempty,max=2,dive,oneof=osm_overpass google_places_ids"`
}

func (s *Server) claimJobs(w http.ResponseWriter, r *http.Request) error {
	var in claimBody
	if err := httpx.Decode(w, r, &in, 4<<10); err != nil {
		return err
	}
	if in.Limit == 0 {
		in.Limit = 4
	}
	jobs, err := store.ClaimJobs(r.Context(), s.Store.Pool, in.WorkerID, in.Limit, in.Providers)
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, map[string]any{"jobs": jobs, "lease_seconds": int(store.JobLease.Seconds())})
	return nil
}

type completeBody struct {
	WorkerID string            `json:"worker_id" validate:"required,max=100"`
	Results  []store.JobResult `json:"results" validate:"max=2000,dive"`
}

func (s *Server) completeJob(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in completeBody
	if err := httpx.Decode(w, r, &in, 8<<20); err != nil {
		return err
	}
	res, err := s.Store.CompleteJob(r.Context(), id, in.WorkerID, in.Results)
	if err != nil {
		return translate(err)
	}
	httpx.JSON(w, 200, res)
	return nil
}

type failBody struct {
	WorkerID string `json:"worker_id" validate:"required,max=100"`
	Error    string `json:"error" validate:"required,max=4000"`
}

func (s *Server) failJob(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in failBody
	if err := httpx.Decode(w, r, &in, 16<<10); err != nil {
		return err
	}
	j, err := store.FailJob(r.Context(), s.Store.Pool, id, in.WorkerID, in.Error)
	if err != nil {
		return translate(err)
	}
	httpx.JSON(w, 200, j)
	return nil
}

func (s *Server) workerMaintenance(w http.ResponseWriter, r *http.Request) error {
	rep, err := s.Store.RunMaintenance(r.Context())
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, rep)
	return nil
}
