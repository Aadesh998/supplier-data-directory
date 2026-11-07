package api

import (
	"net/http"

	"github.com/bharatmfg/directory/internal/httpx"
	"github.com/bharatmfg/directory/internal/store"
)

type loginRequest struct {
	Email    string `json:"email" validate:"required,max=254"`
	Password string `json:"password" validate:"required,max=200"`
}

type sessionResponse struct {
	User      store.AdminUser `json:"user"`
	CSRFToken string          `json:"csrf_token"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	var in loginRequest
	if err := httpx.Decode(w, r, &in, 8<<10); err != nil {
		return err
	}
	if !s.originAllowed(r) {
		return httpx.E(http.StatusForbidden, "csrf_failed", "request origin not allowed")
	}
	u, res, err := store.Authenticate(r.Context(), s.Store.Pool, in.Email, in.Password)
	if err != nil {
		return err
	}
	switch res {
	case store.LoginLocked:
		return httpx.E(http.StatusTooManyRequests, "account_locked", "too many failed attempts; try again in 15 minutes")
	case store.LoginInvalid:
		_ = store.Audit(r.Context(), s.Store.Pool, store.PublicActor(httpx.RequestID(r.Context())), "auth.login_failed", "admin_user", nil, nil, nil)
		return httpx.E(http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
	}
	ipHash := s.Store.Box.HashIP(httpx.ClientIP(r, s.Cfg.TrustProxyHeaders))
	token, sess, err := store.CreateSession(r.Context(), s.Store.Pool, u.ID, ipHash, r.UserAgent(), s.Cfg.SessionTTL)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: s.Cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode, Expires: sess.ExpiresAt})
	a := store.Actor{ID: u.ID, Email: u.Email, Role: u.Role, Type: "admin", RequestID: httpx.RequestID(r.Context())}
	_ = store.Audit(r.Context(), s.Store.Pool, a, "auth.login", "admin_user", &u.ID, nil, nil)
	httpx.JSON(w, 200, sessionResponse{User: u, CSRFToken: sess.CSRFToken})
	return nil
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) error {
	sess := sessionFrom(r.Context())
	if err := store.RevokeSession(r.Context(), s.Store.Pool, sess.ID); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: s.Cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode, MaxAge: -1})
	_ = store.Audit(r.Context(), s.Store.Pool, s.actor(r), "auth.logout", "admin_user", &sess.User.ID, nil, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) error {
	sess := sessionFrom(r.Context())
	httpx.JSON(w, 200, sessionResponse{User: sess.User, CSRFToken: sess.CSRFToken})
	return nil
}
