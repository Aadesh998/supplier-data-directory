// Package api wires HTTP routes to the store.
package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/bharatmfg/directory/internal/config"
	"github.com/bharatmfg/directory/internal/discovery"
	"github.com/bharatmfg/directory/internal/httpx"
	"github.com/bharatmfg/directory/internal/notify"
	"github.com/bharatmfg/directory/internal/store"
	"github.com/bharatmfg/directory/packages/openapi"
)

// Server holds dependencies for HTTP handlers.
type Server struct {
	Cfg       config.Config
	Store     *store.Store
	Log       *slog.Logger
	Mailer    notify.Mailer
	Discovery discovery.Provider // nil when disabled

	loginLimiter      *httpx.RateLimiter
	submissionLimiter *httpx.RateLimiter
	privacyLimiter    *httpx.RateLimiter
	publicLimiter     *httpx.RateLimiter
	adminLimiter      *httpx.RateLimiter
}

// SessionCookie is the admin session cookie name.
const SessionCookie = "bm_session"

const maxJSONBody = 256 << 10

// NewServer creates a Server with default rate limits.
func NewServer(cfg config.Config, st *store.Store, log *slog.Logger, mailer notify.Mailer, disc discovery.Provider) *Server {
	return &Server{
		Cfg: cfg, Store: st, Log: log, Mailer: mailer, Discovery: disc,
		loginLimiter:      httpx.NewRateLimiter(10, time.Minute, 5),
		submissionLimiter: httpx.NewRateLimiter(10, time.Hour, 5),
		privacyLimiter:    httpx.NewRateLimiter(10, time.Hour, 5),
		publicLimiter:     httpx.NewRateLimiter(120, time.Minute, 60),
		adminLimiter:      httpx.NewRateLimiter(600, time.Minute, 120),
	}
}

type h = httpx.Handler

// Routes builds the router.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware, httpx.Recoverer, httpx.Logger(s.Log), httpx.SecurityHeaders, httpx.CORS(s.Cfg.CORSAllowedOrigins))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { httpx.JSON(w, 200, map[string]string{"status": "ok"}) })
	r.Method(http.MethodGet, "/readyz", h(s.readyz))

	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(s.publicLimit)
			r.Method(http.MethodGet, "/categories", h(s.listCategories))
			r.Method(http.MethodGet, "/categories/tree", h(s.categoryTree))
			r.Method(http.MethodGet, "/categories/{slug}", h(s.getCategory))
			r.Method(http.MethodGet, "/states", h(s.listStates))
			r.Method(http.MethodGet, "/roles", h(s.listRoles))
			r.Method(http.MethodGet, "/public/companies", h(s.searchPublic))
			r.Method(http.MethodGet, "/public/companies/{slug}", h(s.getPublic))
			r.Method(http.MethodGet, "/public/sitemap", h(s.sitemap))
		})
		r.With(httpx.Limit(s.submissionLimiter, s.Cfg.TrustProxyHeaders)).Method(http.MethodPost, "/supplier-submissions", h(s.createSubmission))
		r.With(httpx.Limit(s.privacyLimiter, s.Cfg.TrustProxyHeaders)).Method(http.MethodPost, "/privacy-requests", h(s.createPrivacyRequest))
		if !s.Cfg.IsProduction() {
			r.Get("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/yaml")
				_, _ = w.Write(openapi.Spec)
			})
		}

		r.Route("/worker", func(r chi.Router) {
			r.Use(s.requireWorker)
			r.Method(http.MethodPost, "/jobs/claim", h(s.claimJobs))
			r.Method(http.MethodPost, "/jobs/{id}/complete", h(s.completeJob))
			r.Method(http.MethodPost, "/jobs/{id}/fail", h(s.failJob))
			r.Method(http.MethodPost, "/maintenance", h(s.workerMaintenance))
		})

		r.Route("/admin", func(r chi.Router) {
			r.Use(httpx.Limit(s.adminLimiter, s.Cfg.TrustProxyHeaders))
			r.With(httpx.Limit(s.loginLimiter, s.Cfg.TrustProxyHeaders)).Method(http.MethodPost, "/login", h(s.login))
			r.Group(func(r chi.Router) {
				r.Use(s.requireSession)
				r.Method(http.MethodPost, "/logout", h(s.logout))
				r.Method(http.MethodGet, "/me", h(s.me))
				r.Method(http.MethodGet, "/stats", h(s.stats))

				r.Method(http.MethodGet, "/companies", h(s.adminListCompanies))
				r.Method(http.MethodPost, "/companies", s.role(h(s.adminCreateCompany), "owner", "admin"))
				r.Method(http.MethodGet, "/companies/{id}", h(s.adminGetCompany))
				r.Method(http.MethodPatch, "/companies/{id}", s.role(h(s.adminUpdateCompany), "owner", "admin"))
				r.Method(http.MethodPost, "/companies/{id}/facilities", s.role(h(s.adminAddFacility), "owner", "admin"))
				r.Method(http.MethodDelete, "/companies/{id}/facilities/{facilityId}", s.role(h(s.adminRemoveFacility), "owner", "admin"))
				r.Method(http.MethodPost, "/companies/{id}/products", s.role(h(s.adminAddProduct), "owner", "admin"))
				r.Method(http.MethodDelete, "/companies/{id}/products/{productId}", s.role(h(s.adminRemoveProduct), "owner", "admin"))
				r.Method(http.MethodPost, "/companies/{id}/verify", h(s.adminVerify))
				r.Method(http.MethodPost, "/companies/{id}/publish", s.role(h(s.adminPublish), "owner", "admin"))
				r.Method(http.MethodPost, "/companies/{id}/unpublish", s.role(h(s.adminUnpublish), "owner", "admin"))
				r.Method(http.MethodPost, "/companies/{id}/archive", s.role(h(s.adminArchive), "owner", "admin"))

				r.Method(http.MethodGet, "/submissions", h(s.adminListSubmissions))
				r.Method(http.MethodGet, "/submissions/{id}", h(s.adminGetSubmission))
				r.Method(http.MethodPost, "/submissions/{id}/approve", h(s.adminApproveSubmission))
				r.Method(http.MethodPost, "/submissions/{id}/reject", h(s.adminRejectSubmission))

				r.Method(http.MethodGet, "/import-fields", h(s.importFields))
				r.Method(http.MethodGet, "/imports", h(s.listImports))
				r.Method(http.MethodPost, "/imports", s.role(h(s.createImport), "owner", "admin"))
				r.Method(http.MethodGet, "/imports/{id}", h(s.getImport))
				r.Method(http.MethodGet, "/imports/{id}/rows", h(s.listImportRows))
				r.Method(http.MethodPatch, "/imports/{id}/rows/{rowId}", s.role(h(s.updateImportRow), "owner", "admin"))
				r.Method(http.MethodPost, "/imports/{id}/mapping", s.role(h(s.remapImport), "owner", "admin"))
				r.Method(http.MethodPost, "/imports/{id}/commit", s.role(h(s.commitImport), "owner", "admin"))
				r.Method(http.MethodPost, "/imports/{id}/rollback", s.role(h(s.rollbackImport), "owner", "admin"))
				r.Method(http.MethodGet, "/imports/{id}/report.csv", h(s.importReport))

				r.Method(http.MethodGet, "/duplicates", h(s.listDuplicates))
				r.Method(http.MethodPost, "/duplicates/scan", s.role(h(s.scanDuplicates), "owner", "admin"))
				r.Method(http.MethodPost, "/duplicates/{id}/resolve", s.role(h(s.resolveDuplicate), "owner", "admin"))

				r.Method(http.MethodGet, "/categories", h(s.adminListCategories))
				r.Method(http.MethodPost, "/categories", s.role(h(s.adminCreateCategory), "owner", "admin"))
				r.Method(http.MethodPatch, "/categories/{id}", s.role(h(s.adminUpdateCategory), "owner", "admin"))
				r.Method(http.MethodGet, "/sources", h(s.listSources))
				r.Method(http.MethodPost, "/sources", s.role(h(s.createSource), "owner", "admin"))
				r.Method(http.MethodPatch, "/sources/{id}", s.role(h(s.updateSource), "owner", "admin"))
				r.Method(http.MethodGet, "/audit-logs", h(s.auditLogs))
				r.Method(http.MethodPost, "/exports", s.role(h(s.export), "owner", "admin"))
				r.Method(http.MethodGet, "/privacy-requests", h(s.listPrivacyRequests))
				r.Method(http.MethodPost, "/privacy-requests/{id}/resolve", s.role(h(s.resolvePrivacyRequest), "owner", "admin"))
				r.Method(http.MethodGet, "/users", s.role(h(s.listUsers), "owner"))
				r.Method(http.MethodPost, "/users", s.role(h(s.createUser), "owner"))

				r.Method(http.MethodGet, "/discovery/status", h(s.discoveryStatus))
				r.Method(http.MethodPost, "/discovery/search", s.role(h(s.discoverySearch), "owner", "admin"))
				r.Method(http.MethodGet, "/discovery/leads", h(s.listLeads))
				r.Method(http.MethodPost, "/discovery/leads", s.role(h(s.saveLead), "owner", "admin"))
				r.Method(http.MethodPatch, "/discovery/leads/{id}", s.role(h(s.updateLead), "owner", "admin"))

				r.Method(http.MethodGet, "/discovery/jobs", h(s.listJobs))
				r.Method(http.MethodGet, "/discovery/jobs/stats", h(s.jobStats))
				r.Method(http.MethodPost, "/discovery/jobs/upload", s.role(h(s.uploadJobs), "owner", "admin"))
				r.Method(http.MethodPost, "/discovery/jobs/actions", s.role(h(s.jobAction), "owner", "admin"))
				r.Method(http.MethodGet, "/discovery/results", h(s.listResults))
				r.Method(http.MethodPatch, "/discovery/results/{id}", h(s.updateResult))
				r.Method(http.MethodPost, "/discovery/results/{id}/promote", s.role(h(s.promoteResult), "owner", "admin"))
			})
		})
	})
	r.NotFound(func(w http.ResponseWriter, r *http.Request) { httpx.WriteError(w, r, httpx.ErrNotFound) })
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, r, httpx.E(http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed"))
	})
	return r
}

// publicLimit applies the per-IP public limit, except for the web server's own server-side
// requests authenticated with INTERNAL_API_TOKEN (they would otherwise share one IP bucket).
func (s *Server) publicLimit(next http.Handler) http.Handler {
	limited := httpx.Limit(s.publicLimiter, s.Cfg.TrustProxyHeaders)(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.isInternal(r) {
			next.ServeHTTP(w, r)
			return
		}
		limited.ServeHTTP(w, r)
	})
}

func (s *Server) isInternal(r *http.Request) bool {
	tok := r.Header.Get("X-Internal-Token")
	return s.Cfg.InternalAPIToken != "" && tok != "" &&
		subtle.ConstantTimeCompare([]byte(tok), []byte(s.Cfg.InternalAPIToken)) == 1
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.Store.Pool.Ping(ctx); err != nil {
		return httpx.E(http.StatusServiceUnavailable, "not_ready", "database unavailable")
	}
	httpx.JSON(w, 200, map[string]string{"status": "ready"})
	return nil
}

type ctxKey int

const sessionKey ctxKey = iota

func sessionFrom(ctx context.Context) store.Session {
	s, _ := ctx.Value(sessionKey).(store.Session)
	return s
}

func (s *Server) actor(r *http.Request) store.Actor {
	sess := sessionFrom(r.Context())
	return store.Actor{ID: sess.User.ID, Email: sess.User.Email, Role: sess.User.Role, Type: "admin", RequestID: httpx.RequestID(r.Context())}
}

// requireSession authenticates the session cookie and enforces CSRF + origin checks on mutations.
func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(SessionCookie)
		if err != nil || c.Value == "" {
			httpx.WriteError(w, r, httpx.ErrUnauthorized)
			return
		}
		sess, err := store.LookupSession(r.Context(), s.Store.Pool, c.Value)
		if err != nil {
			if store.IsNotFound(err) {
				httpx.WriteError(w, r, httpx.E(http.StatusUnauthorized, "unauthorized", "session expired, please sign in again"))
			} else {
				httpx.WriteError(w, r, err)
			}
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if !s.originAllowed(r) {
				httpx.WriteError(w, r, httpx.E(http.StatusForbidden, "csrf_failed", "request origin not allowed"))
				return
			}
			tok := r.Header.Get("X-CSRF-Token")
			if tok == "" || subtle.ConstantTimeCompare([]byte(tok), []byte(sess.CSRFToken)) != 1 {
				httpx.WriteError(w, r, httpx.E(http.StatusForbidden, "csrf_failed", "missing or invalid CSRF token"))
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey, sess)))
	})
}

// originAllowed accepts requests with no Origin/Referer (non-browser clients carry no ambient
// cookie risk beyond the CSRF token) or whose origin is allowlisted or matches the public site.
func (s *Server) originAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		if ref := r.Header.Get("Referer"); ref != "" {
			if u, err := url.Parse(ref); err == nil {
				origin = u.Scheme + "://" + u.Host
			}
		}
	}
	if origin == "" {
		return true
	}
	return slices.Contains(s.Cfg.CORSAllowedOrigins, origin) || origin == s.Cfg.PublicSiteURL
}

// role restricts a handler to the listed roles.
func (s *Server) role(next http.Handler, roles ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !slices.Contains(roles, sessionFrom(r.Context()).User.Role) {
			httpx.WriteError(w, r, httpx.ErrForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// translate maps store errors to API errors.
func translate(err error) error {
	if err == nil {
		return nil
	}
	var inv store.ErrInvalid
	var conf store.ErrConflict
	var pol store.ErrPolicy
	switch {
	case errors.As(err, &inv):
		if len(inv.Fields) > 0 {
			return httpx.FieldErrors(inv.Fields)
		}
		return httpx.E(http.StatusUnprocessableEntity, "validation_failed", inv.Msg)
	case errors.As(err, &conf):
		return httpx.E(http.StatusConflict, "conflict", conf.Msg)
	case errors.As(err, &pol):
		e := httpx.E(http.StatusUnprocessableEntity, pol.Code, pol.Msg)
		if len(pol.Reasons) > 0 {
			e = e.WithDetails(map[string]any{"reasons": pol.Reasons})
		}
		return e
	}
	return err
}

func validUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
				return false
			}
		}
	}
	return true
}

// pathID reads and validates a UUID path parameter.
func pathID(r *http.Request, name string) (string, error) {
	v := chi.URLParam(r, name)
	if !validUUID(v) {
		return "", httpx.ErrNotFound
	}
	return strings.ToLower(v), nil
}
