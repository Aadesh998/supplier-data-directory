package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/bharatmfg/directory/internal/httpx"
	"github.com/bharatmfg/directory/internal/notify"
	"github.com/bharatmfg/directory/internal/store"
)

func publicCache(w http.ResponseWriter) {
	// Short cache so unpublished records disappear quickly from shared caches.
	w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=60")
}

func (s *Server) listCategories(w http.ResponseWriter, r *http.Request) error {
	cats, err := store.ListCategories(r.Context(), s.Store.Pool, false)
	if err != nil {
		return err
	}
	publicCache(w)
	httpx.JSON(w, 200, map[string]any{"data": cats})
	return nil
}

func (s *Server) categoryTree(w http.ResponseWriter, r *http.Request) error {
	cats, err := store.ListCategories(r.Context(), s.Store.Pool, false)
	if err != nil {
		return err
	}
	publicCache(w)
	httpx.JSON(w, 200, map[string]any{"data": store.BuildTree(cats)})
	return nil
}

func (s *Server) getCategory(w http.ResponseWriter, r *http.Request) error {
	d, err := store.GetCategoryDetail(r.Context(), s.Store.Pool, chi.URLParam(r, "slug"))
	if err != nil {
		return err
	}
	publicCache(w)
	httpx.JSON(w, 200, d)
	return nil
}

func (s *Server) listStates(w http.ResponseWriter, r *http.Request) error {
	st, err := store.ListStates(r.Context(), s.Store.Pool)
	if err != nil {
		return err
	}
	publicCache(w)
	httpx.JSON(w, 200, map[string]any{"data": st})
	return nil
}

func (s *Server) listRoles(w http.ResponseWriter, r *http.Request) error {
	roles, err := store.ListRoles(r.Context(), s.Store.Pool)
	if err != nil {
		return err
	}
	publicCache(w)
	httpx.JSON(w, 200, map[string]any{"data": roles})
	return nil
}

func (s *Server) searchPublic(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	f := store.PublicFilter{Q: q.Get("q"), Category: q.Get("category"), State: q.Get("state"), City: q.Get("city"), Role: q.Get("role")}
	if len(f.Q) > 100 || len(f.City) > 100 || len(f.Category) > 160 || len(f.State) > 60 || len(f.Role) > 40 {
		return httpx.FieldErrors(map[string]string{"q": "search parameters are too long"})
	}
	p := httpx.ParsePagination(r, 20)
	data, total, err := store.SearchPublicCompanies(r.Context(), s.Store.Pool, f, p.Limit, p.Offset())
	if err != nil {
		return err
	}
	publicCache(w)
	httpx.JSON(w, 200, httpx.NewPaged(data, p, total))
	return nil
}

func (s *Server) getPublic(w http.ResponseWriter, r *http.Request) error {
	slug := chi.URLParam(r, "slug")
	if len(slug) > 160 {
		return httpx.ErrNotFound
	}
	res, err := store.GetPublicCompany(r.Context(), s.Store.Pool, slug)
	if err != nil {
		return err
	}
	switch {
	case res.Company != nil:
		res.Company.Teaser()
		publicCache(w)
		httpx.JSON(w, 200, res.Company)
	case res.RedirectSlug != "":
		w.Header().Set("Location", "/api/v1/public/companies/"+res.RedirectSlug)
		httpx.JSON(w, http.StatusMovedPermanently, map[string]string{"redirect_slug": res.RedirectSlug})
	case res.Gone:
		return httpx.E(http.StatusGone, "gone", "this profile has been removed")
	default:
		return httpx.ErrNotFound
	}
	return nil
}

func (s *Server) sitemap(w http.ResponseWriter, r *http.Request) error {
	m, err := store.Sitemap(r.Context(), s.Store.Pool)
	if err != nil {
		return err
	}
	publicCache(w)
	httpx.JSON(w, 200, m)
	return nil
}

const (
	minFormSeconds = 3
	maxFormAge     = 24 * time.Hour
	maxPerEmailDay = 3
)

func (s *Server) createSubmission(w http.ResponseWriter, r *http.Request) error {
	var p store.SubmissionPayload
	if err := httpx.Decode(w, r, &p, maxJSONBody); err != nil {
		return err
	}
	reqID := httpx.RequestID(r.Context())
	accepted := func(id string) {
		httpx.JSON(w, http.StatusAccepted, map[string]string{"id": id, "status": "submitted",
			"message": "Thank you. Your registration will be reviewed by a person before anything is published."})
	}
	// Anti-spam: honeypot and implausibly fast completion are silently accepted and dropped,
	// so bots get no signal.
	started := time.UnixMilli(p.FormStartedAt)
	if p.WebsiteConfirm != "" || time.Since(started) < minFormSeconds*time.Second {
		s.Log.InfoContext(r.Context(), "submission dropped by anti-spam checks", "request_id", reqID)
		accepted("00000000-0000-0000-0000-000000000000")
		return nil
	}
	if time.Since(started) > maxFormAge || started.After(time.Now().Add(time.Minute)) {
		return httpx.E(http.StatusUnprocessableEntity, "form_expired", "this form has expired; please reload the page and submit again")
	}
	if err := p.ValidateSemantics(r.Context(), s.Store.Pool, s.Cfg.PrivacyNoticeVersion); err != nil {
		return translate(err)
	}
	if n, err := s.Store.CountRecentSubmissions(r.Context(), p.Submitter.Email, 24*time.Hour); err != nil {
		return err
	} else if n >= maxPerEmailDay {
		return httpx.E(http.StatusTooManyRequests, "rate_limited", "too many submissions from this email today; please try again tomorrow")
	}
	key := r.Header.Get("Idempotency-Key")
	if key != "" && (len(key) < 8 || len(key) > 128) {
		return httpx.E(http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key must be 8-128 characters")
	}
	ipHash := s.Store.Box.HashIP(httpx.ClientIP(r, s.Cfg.TrustProxyHeaders))
	id, repeat, err := s.Store.CreateSubmission(r.Context(), p, key, ipHash, reqID)
	if err != nil {
		return translate(err)
	}
	if !repeat {
		base := s.mailData()
		base["CompanyName"] = p.Company.DisplayName
		base["SubmitterName"] = p.Submitter.Name
		base["SubmissionID"] = id
		base["NoticeVersion"] = p.PrivacyNoticeVersion
		base["SubmittedAt"] = time.Now().UTC().Format("2 Jan 2006 15:04 MST")
		base["PublicListing"] = p.Consents.PublicListing
		base["CommercialAPI"] = p.Consents.CommercialAPI
		notify.Async(s.Log, s.Mailer, notify.Message{To: p.Submitter.Email, Template: "submission_received", Data: base})
		if s.Cfg.AdminNotify != "" {
			notify.Async(s.Log, s.Mailer, notify.Message{To: s.Cfg.AdminNotify, Template: "admin_new_submission", Data: base})
		}
	}
	accepted(id)
	return nil
}

func (s *Server) mailData() map[string]any {
	return map[string]any{"SiteName": "Peoplvance", "SiteURL": s.Cfg.PublicSiteURL}
}

var requestTypeLabels = map[string]string{
	"correction": "correction", "removal": "removal", "withdraw_public_listing": "withdraw public listing",
	"withdraw_commercial_api": "withdraw commercial/API permission", "access": "data access", "other": "privacy",
}

func (s *Server) createPrivacyRequest(w http.ResponseWriter, r *http.Request) error {
	var in store.PrivacyRequestInput
	if err := httpx.Decode(w, r, &in, 64<<10); err != nil {
		return err
	}
	if in.WebsiteConfirm != "" {
		httpx.JSON(w, http.StatusAccepted, map[string]string{"id": "00000000-0000-0000-0000-000000000000", "status": "received"})
		return nil
	}
	id, err := s.Store.CreatePrivacyRequest(r.Context(), in, httpx.RequestID(r.Context()))
	if err != nil {
		return translate(err)
	}
	d := s.mailData()
	d["RequesterName"], d["RequestID"], d["RequestTypeLabel"] = in.RequesterName, id, requestTypeLabels[in.RequestType]
	d["ReceivedAt"] = time.Now().UTC().Format("2 Jan 2006")
	notify.Async(s.Log, s.Mailer, notify.Message{To: in.Email, Template: "privacy_request_received", Data: d})
	httpx.JSON(w, http.StatusAccepted, map[string]string{"id": id, "status": "received",
		"message": "We received your request and will reply by email."})
	return nil
}
