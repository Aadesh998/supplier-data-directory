package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bharatmfg/directory/internal/discovery"
	"github.com/bharatmfg/directory/internal/httpx"
	"github.com/bharatmfg/directory/internal/store"
)

// --- duplicates ---

func (s *Server) listDuplicates(w http.ResponseWriter, r *http.Request) error {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "open"
	}
	p := httpx.ParsePagination(r, 25)
	data, total, err := store.ListDuplicatePairs(r.Context(), s.Store.Pool, status, p.Limit, p.Offset())
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, httpx.NewPaged(data, p, total))
	return nil
}

func (s *Server) scanDuplicates(w http.ResponseWriter, r *http.Request) error {
	n, err := s.Store.ScanDuplicates(r.Context(), s.actor(r))
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, map[string]int{"created": n})
	return nil
}

type resolveBody struct {
	Action string `json:"action" validate:"required,oneof=merge dismiss"`
	Keep   string `json:"keep" validate:"omitempty,oneof=a b"`
	Notes  string `json:"notes" validate:"max=2000"`
}

func (s *Server) resolveDuplicate(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in resolveBody
	if err := httpx.Decode(w, r, &in, 8<<10); err != nil {
		return err
	}
	if in.Keep == "" {
		in.Keep = "a"
	}
	if err := s.Store.ResolveDuplicate(r.Context(), s.actor(r), id, in.Action, in.Keep, in.Notes); err != nil {
		return translate(err)
	}
	d, err := store.GetDuplicatePair(r.Context(), s.Store.Pool, id)
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, d)
	return nil
}

// --- taxonomy ---

func (s *Server) adminListCategories(w http.ResponseWriter, r *http.Request) error {
	cats, err := store.ListCategories(r.Context(), s.Store.Pool, true)
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, map[string]any{"data": cats})
	return nil
}

func (s *Server) adminCreateCategory(w http.ResponseWriter, r *http.Request) error {
	var in store.CategoryInput
	if err := httpx.Decode(w, r, &in, 32<<10); err != nil {
		return err
	}
	c, err := s.Store.CreateCategory(r.Context(), s.actor(r), in)
	if err != nil {
		return translate(err)
	}
	httpx.JSON(w, http.StatusCreated, c)
	return nil
}

func (s *Server) adminUpdateCategory(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in store.CategoryInput
	if err := httpx.Decode(w, r, &in, 32<<10); err != nil {
		return err
	}
	c, err := s.Store.UpdateCategory(r.Context(), s.actor(r), id, in)
	if err != nil {
		return translate(err)
	}
	httpx.JSON(w, 200, c)
	return nil
}

// --- sources ---

// checkSourcePermissionRole: only the owner may grant commercial redistribution rights.
func (s *Server) checkSourcePermissionRole(r *http.Request, in store.SourceInput) error {
	if in.AllowedCommercialAPI != nil && *in.AllowedCommercialAPI && sessionFrom(r.Context()).User.Role != "owner" {
		return httpx.E(http.StatusForbidden, "forbidden", "only the owner can declare commercial/API redistribution rights")
	}
	return nil
}

func (s *Server) listSources(w http.ResponseWriter, r *http.Request) error {
	p := httpx.ParsePagination(r, 50)
	data, total, err := store.ListSources(r.Context(), s.Store.Pool, p.Limit, p.Offset())
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, httpx.NewPaged(data, p, total))
	return nil
}

func (s *Server) createSource(w http.ResponseWriter, r *http.Request) error {
	var in store.SourceInput
	if err := httpx.Decode(w, r, &in, 32<<10); err != nil {
		return err
	}
	if err := s.checkSourcePermissionRole(r, in); err != nil {
		return err
	}
	d, err := store.CreateSource(r.Context(), s.Store.Pool, s.actor(r), in)
	if err != nil {
		return translate(err)
	}
	httpx.JSON(w, http.StatusCreated, d)
	return nil
}

func (s *Server) updateSource(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in store.SourceInput
	if err := httpx.Decode(w, r, &in, 32<<10); err != nil {
		return err
	}
	changingPerms := in.AllowedInternalUse != nil || in.AllowedPublicDisplay != nil || in.AllowedCommercialAPI != nil || in.RetentionExpiresAt != nil
	if changingPerms && sessionFrom(r.Context()).User.Role != "owner" {
		return httpx.E(http.StatusForbidden, "forbidden", "only the owner can change a source's permissions or retention")
	}
	d, unpublished, err := s.Store.UpdateSource(r.Context(), s.actor(r), id, in)
	if err != nil {
		return translate(err)
	}
	if len(unpublished) > 0 {
		w.Header().Set("X-Unpublished-Count", strconv.Itoa(len(unpublished)))
	}
	httpx.JSON(w, 200, d)
	return nil
}

// --- audit ---

type auditRow struct {
	ID         string          `json:"id"`
	ActorEmail *string         `json:"actor_email"`
	ActorType  string          `json:"actor_type"`
	Action     string          `json:"action"`
	EntityType string          `json:"entity_type"`
	EntityID   *string         `json:"entity_id"`
	Before     json.RawMessage `json:"before"`
	After      json.RawMessage `json:"after"`
	RequestID  *string         `json:"request_id"`
	CreatedAt  time.Time       `json:"created_at"`
}

func (s *Server) auditLogs(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	p := httpx.ParsePagination(r, 50)
	entityID := q.Get("entity_id")
	if entityID != "" && !validUUID(entityID) {
		return httpx.FieldErrors(map[string]string{"entity_id": "must be a UUID"})
	}
	where := `($1 = '' OR l.entity_type = $1) AND ($2 = '' OR l.entity_id = NULLIF($2,'')::uuid) AND ($3 = '' OR l.action LIKE $3 || '%')`
	args := []any{q.Get("entity_type"), entityID, q.Get("action")}
	var total int
	if err := s.Store.Pool.QueryRow(r.Context(), `SELECT count(*) FROM audit_logs l WHERE `+where, args...).Scan(&total); err != nil {
		return err
	}
	rows, err := s.Store.Pool.Query(r.Context(), `SELECT l.id, u.email::text, l.actor_type, l.action, l.entity_type, l.entity_id,
		l.before, l.after, l.request_id, l.created_at FROM audit_logs l LEFT JOIN admin_users u ON u.id = l.actor_id
		WHERE `+where+` ORDER BY l.created_at DESC LIMIT $4 OFFSET $5`, append(args, p.Limit, p.Offset())...)
	if err != nil {
		return err
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (auditRow, error) {
		var a auditRow
		var before, after []byte
		err := row.Scan(&a.ID, &a.ActorEmail, &a.ActorType, &a.Action, &a.EntityType, &a.EntityID, &before, &after, &a.RequestID, &a.CreatedAt)
		if before != nil {
			a.Before = before
		}
		if after != nil {
			a.After = after
		}
		return a, err
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, httpx.NewPaged(out, p, total))
	return nil
}

// --- exports ---

type exportBody struct {
	Scope             string `json:"scope" validate:"required,oneof=internal public commercial"`
	PublicationStatus string `json:"publication_status" validate:"omitempty,oneof=draft submitted under_review approved_private published rejected needs_changes archived"`
}

func (s *Server) export(w http.ResponseWriter, r *http.Request) error {
	var in exportBody
	if err := httpx.Decode(w, r, &in, 8<<10); err != nil {
		return err
	}
	if in.Scope == "commercial" && sessionFrom(r.Context()).User.Role != "owner" {
		return httpx.E(http.StatusForbidden, "forbidden", "only the owner can run commercial exports")
	}
	b, n, err := s.Store.ExportCSV(r.Context(), s.actor(r), store.Purpose(in.Scope), in.PublicationStatus)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="export-`+in.Scope+`-`+time.Now().UTC().Format("20060102-150405")+`.csv"`)
	w.Header().Set("X-Export-Rows", strconv.Itoa(n))
	_, _ = w.Write(b)
	return nil
}

// --- privacy requests ---

func (s *Server) listPrivacyRequests(w http.ResponseWriter, r *http.Request) error {
	p := httpx.ParsePagination(r, 25)
	data, total, err := s.Store.ListPrivacyRequests(r.Context(), r.URL.Query().Get("status"), p.Limit, p.Offset())
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, httpx.NewPaged(data, p, total))
	return nil
}

func (s *Server) resolvePrivacyRequest(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in store.ResolvePrivacyInput
	if err := httpx.Decode(w, r, &in, 16<<10); err != nil {
		return err
	}
	if in.Action == "erase_company" && sessionFrom(r.Context()).User.Role != "owner" {
		return httpx.E(http.StatusForbidden, "forbidden", "only the owner can permanently erase a company")
	}
	pr, err := s.Store.ResolvePrivacyRequest(r.Context(), s.actor(r), id, in)
	if err != nil {
		return translate(err)
	}
	httpx.JSON(w, 200, pr)
	return nil
}

// --- users ---

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) error {
	users, err := store.ListAdminUsers(r.Context(), s.Store.Pool)
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, map[string]any{"data": users})
	return nil
}

type createUserBody struct {
	Email       string `json:"email" validate:"required,email,max=254"`
	DisplayName string `json:"display_name" validate:"max=120"`
	Password    string `json:"password" validate:"required,min=12,max=200"`
	Role        string `json:"role" validate:"required,oneof=admin reviewer"`
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) error {
	var in createUserBody
	if err := httpx.Decode(w, r, &in, 8<<10); err != nil {
		return err
	}
	u, err := store.CreateAdminUser(r.Context(), s.Store.Pool, s.actor(r), in.Email, in.DisplayName, in.Password, in.Role)
	if err != nil {
		return translate(err)
	}
	httpx.JSON(w, http.StatusCreated, u)
	return nil
}

// --- discovery ---

func (s *Server) discoveryStatus(w http.ResponseWriter, r *http.Request) error {
	if s.Discovery == nil {
		reason := "Discovery is disabled by default. It requires GOOGLE_PLACES_ENABLED=true, an API key with billing alerts and quotas, and a review of Google's current Maps Platform terms (see docs/data-source-policy.md)."
		httpx.JSON(w, 200, map[string]any{"enabled": false, "provider": "google_places", "reason": reason})
		return nil
	}
	httpx.JSON(w, 200, map[string]any{"enabled": true, "provider": s.Discovery.Name(), "policy": s.Discovery.RetentionPolicy()})
	return nil
}

type discoveryBody struct {
	Query    string `json:"query" validate:"required,min=2,max=120"`
	Location string `json:"location" validate:"max=120"`
}

func (s *Server) discoverySearch(w http.ResponseWriter, r *http.Request) error {
	if s.Discovery == nil {
		return httpx.E(http.StatusServiceUnavailable, "discovery_disabled", "discovery is disabled")
	}
	var in discoveryBody
	if err := httpx.Decode(w, r, &in, 4<<10); err != nil {
		return err
	}
	res, err := s.Discovery.SearchBusinesses(r.Context(), in.Query, in.Location)
	if errors.Is(err, discovery.ErrQuotaExceeded) {
		return httpx.E(http.StatusTooManyRequests, "quota_exceeded", "the daily discovery cap has been reached")
	}
	if err != nil {
		s.Log.ErrorContext(r.Context(), "discovery search failed", "err", err)
		return httpx.E(http.StatusBadGateway, "provider_error", "the discovery provider returned an error")
	}
	// Results are returned transiently and never persisted.
	_ = store.Audit(r.Context(), s.Store.Pool, s.actor(r), "discovery.search", "discovery", nil, nil,
		map[string]any{"query": in.Query, "location": in.Location, "results": len(res)})
	pol := s.Discovery.RetentionPolicy()
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, 200, map[string]any{"results": res, "attribution": pol.Attribution, "notice": pol.Notice})
	return nil
}

func (s *Server) listLeads(w http.ResponseWriter, r *http.Request) error {
	p := httpx.ParsePagination(r, 50)
	data, total, err := store.ListLeads(r.Context(), s.Store.Pool, p.Limit, p.Offset())
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, httpx.NewPaged(data, p, total))
	return nil
}

type leadBody struct {
	ProviderPlaceID string `json:"provider_place_id" validate:"required,max=512"`
	SearchQuery     string `json:"search_query" validate:"max=240"`
	Notes           string `json:"notes" validate:"max=2000"`
}

func (s *Server) saveLead(w http.ResponseWriter, r *http.Request) error {
	var in leadBody
	if err := httpx.Decode(w, r, &in, 8<<10); err != nil {
		return err
	}
	if strings.ContainsAny(in.ProviderPlaceID, " /\\") {
		return httpx.FieldErrors(map[string]string{"provider_place_id": "is not a valid place id"})
	}
	l, err := store.SaveLead(r.Context(), s.Store.Pool, s.actor(r), "google_places", in.ProviderPlaceID, in.SearchQuery, in.Notes)
	if err != nil {
		return translate(err)
	}
	httpx.JSON(w, http.StatusCreated, l)
	return nil
}

type leadUpdate struct {
	Status *string `json:"status" validate:"omitempty,oneof=new invited registered dismissed"`
	Notes  *string `json:"notes" validate:"omitempty,max=2000"`
}

func (s *Server) updateLead(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in leadUpdate
	if err := httpx.Decode(w, r, &in, 8<<10); err != nil {
		return err
	}
	l, err := store.UpdateLead(r.Context(), s.Store.Pool, s.actor(r), id, in.Status, in.Notes)
	if err != nil {
		return translate(err)
	}
	httpx.JSON(w, 200, l)
	return nil
}
