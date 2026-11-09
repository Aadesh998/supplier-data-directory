package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bharatmfg/directory/internal/normalize"
	"github.com/bharatmfg/directory/internal/secure"
)

// AdminUser is an admin account (never includes the password hash in JSON).
type AdminUser struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	DisplayName  string     `json:"display_name"`
	Role         string     `json:"role"`
	Active       bool       `json:"active"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	passwordHash *string
	lockedUntil  *time.Time
}

const userCols = `id, email::text, display_name, role, active, last_login_at, password_hash, locked_until`

func scanUser(r pgx.Row) (AdminUser, error) {
	var u AdminUser
	err := r.Scan(&u.ID, &u.Email, &u.DisplayName, &u.Role, &u.Active, &u.LastLoginAt, &u.passwordHash, &u.lockedUntil)
	return u, err
}

// CreateAdminUser creates an admin with an Argon2id password hash.
func CreateAdminUser(ctx context.Context, q DBTX, a Actor, email, displayName, password, role string) (AdminUser, error) {
	e, ok := normalize.Email(email)
	if !ok {
		return AdminUser{}, ErrInvalid{Msg: "invalid email", Fields: map[string]string{"email": "must be a valid email address"}}
	}
	if len(password) < 12 {
		return AdminUser{}, ErrInvalid{Msg: "weak password", Fields: map[string]string{"password": "must be at least 12 characters"}}
	}
	hash, err := secure.HashPassword(password)
	if err != nil {
		return AdminUser{}, err
	}
	u, err := scanUser(q.QueryRow(ctx, `INSERT INTO admin_users (email, display_name, password_hash, role) VALUES ($1,$2,$3,$4)
		RETURNING `+userCols, e, displayName, hash, role))
	if err != nil {
		return u, err
	}
	return u, Audit(ctx, q, a, "admin_user.create", "admin_user", &u.ID, nil, map[string]any{"role": role})
}

// ListAdminUsers lists admins.
func ListAdminUsers(ctx context.Context, q DBTX) ([]AdminUser, error) {
	return collect(ctx, q, `SELECT `+userCols+` FROM admin_users ORDER BY created_at`, func(r pgx.Rows) (AdminUser, error) { return scanUser(r) })
}

// LoginResult reports the outcome of a credential check.
type LoginResult int

const (
	LoginOK LoginResult = iota
	LoginInvalid
	LoginLocked
)

const (
	maxFailedLogins = 5
	lockoutDuration = 15 * time.Minute
)

// Authenticate verifies credentials with lockout after repeated failures. Timing is equalised for
// unknown emails.
func Authenticate(ctx context.Context, q DBTX, email, password string) (AdminUser, LoginResult, error) {
	e, _ := normalize.Email(email)
	u, err := scanUser(q.QueryRow(ctx, `SELECT `+userCols+` FROM admin_users WHERE email = $1`, e))
	if IsNotFound(err) {
		secure.VerifyPassword(password, secure.DummyHash)
		return AdminUser{}, LoginInvalid, nil
	}
	if err != nil {
		return AdminUser{}, LoginInvalid, err
	}
	if u.lockedUntil != nil && u.lockedUntil.After(time.Now()) {
		secure.VerifyPassword(password, secure.DummyHash)
		return AdminUser{}, LoginLocked, nil
	}
	if !u.Active || u.passwordHash == nil || !secure.VerifyPassword(password, *u.passwordHash) {
		_, err := q.Exec(ctx, `UPDATE admin_users SET
			failed_login_count = CASE WHEN failed_login_count + 1 >= $2 THEN 0 ELSE failed_login_count + 1 END,
			locked_until = CASE WHEN failed_login_count + 1 >= $2 THEN now() + make_interval(mins => $3) ELSE locked_until END
			WHERE id = $1`, u.ID, maxFailedLogins, int(lockoutDuration.Minutes()))
		return AdminUser{}, LoginInvalid, err
	}
	_, err = q.Exec(ctx, `UPDATE admin_users SET failed_login_count = 0, locked_until = NULL, last_login_at = now() WHERE id = $1`, u.ID)
	return u, LoginOK, err
}

// Session is an authenticated admin session.
type Session struct {
	ID        string
	User      AdminUser
	CSRFToken string
	ExpiresAt time.Time
}

// CreateSession issues a session; the returned token goes in the cookie and only its hash is stored.
func CreateSession(ctx context.Context, q DBTX, userID, ipHash, userAgent string, ttl time.Duration) (token string, s Session, err error) {
	token = secure.RandomToken(32)
	csrf := secure.RandomToken(24)
	if len(userAgent) > 300 {
		userAgent = userAgent[:300]
	}
	err = q.QueryRow(ctx, `INSERT INTO admin_sessions (admin_user_id, token_hash, csrf_token, ip_hash, user_agent, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, expires_at`, userID, secure.SHA256(token), csrf, ipHash, userAgent, time.Now().Add(ttl)).
		Scan(&s.ID, &s.ExpiresAt)
	s.CSRFToken = csrf
	return token, s, err
}

// LookupSession resolves a cookie token to an active session and user.
func LookupSession(ctx context.Context, q DBTX, token string) (Session, error) {
	var s Session
	var lastSeen time.Time
	err := q.QueryRow(ctx, `SELECT s.id, s.csrf_token, s.expires_at, s.last_seen_at, u.id, u.email::text, u.display_name, u.role, u.active, u.last_login_at
		FROM admin_sessions s JOIN admin_users u ON u.id = s.admin_user_id
		WHERE s.token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > now() AND u.active`, secure.SHA256(token)).
		Scan(&s.ID, &s.CSRFToken, &s.ExpiresAt, &lastSeen, &s.User.ID, &s.User.Email, &s.User.DisplayName, &s.User.Role, &s.User.Active, &s.User.LastLoginAt)
	if err != nil {
		return s, err
	}
	if time.Since(lastSeen) > time.Minute {
		_, _ = q.Exec(ctx, `UPDATE admin_sessions SET last_seen_at = now() WHERE id = $1`, s.ID)
	}
	return s, nil
}

// RevokeSession ends a session.
func RevokeSession(ctx context.Context, q DBTX, id string) error {
	_, err := q.Exec(ctx, `UPDATE admin_sessions SET revoked_at = now() WHERE id = $1`, id)
	return err
}

// SetPassword replaces an admin's password, clears lockout and revokes their sessions.
func SetPassword(ctx context.Context, q DBTX, a Actor, email, password string) error {
	if len(password) < 12 {
		return ErrInvalid{Msg: "weak password", Fields: map[string]string{"password": "must be at least 12 characters"}}
	}
	e, _ := normalize.Email(email)
	hash, err := secure.HashPassword(password)
	if err != nil {
		return err
	}
	var id string
	if err := q.QueryRow(ctx, `UPDATE admin_users SET password_hash = $2, failed_login_count = 0, locked_until = NULL
		WHERE email = $1 RETURNING id`, e, hash).Scan(&id); err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `UPDATE admin_sessions SET revoked_at = now() WHERE admin_user_id = $1 AND revoked_at IS NULL`, id); err != nil {
		return err
	}
	return Audit(ctx, q, a, "admin_user.set_password", "admin_user", &id, nil, nil)
}

// AdminUserPatch changes an admin's profile, role or active flag (nil = unchanged).
type AdminUserPatch struct {
	DisplayName *string
	Role        *string
	Active      *bool
}

// UpdateAdminUser applies a patch. An actor cannot demote or deactivate themselves, and the last
// active owner cannot be demoted or deactivated. Deactivation revokes the user's sessions.
func UpdateAdminUser(ctx context.Context, q DBTX, a Actor, id string, p AdminUserPatch) (AdminUser, error) {
	before, err := scanUser(q.QueryRow(ctx, `SELECT `+userCols+` FROM admin_users WHERE id = $1 FOR UPDATE`, id))
	if err != nil {
		return AdminUser{}, err
	}
	demoting := p.Role != nil && *p.Role != before.Role && before.Role == "owner"
	deactivating := p.Active != nil && !*p.Active && before.Active
	if (demoting || deactivating) && id == a.ID {
		return AdminUser{}, ErrConflict{Msg: "you cannot demote or deactivate your own account"}
	}
	if (demoting || deactivating) && before.Role == "owner" {
		var owners int
		if err := q.QueryRow(ctx, `SELECT count(*) FROM admin_users WHERE role = 'owner' AND active AND id <> $1`, id).Scan(&owners); err != nil {
			return AdminUser{}, err
		}
		if owners == 0 {
			return AdminUser{}, ErrConflict{Msg: "at least one active owner must remain"}
		}
	}
	u, err := scanUser(q.QueryRow(ctx, `UPDATE admin_users SET
		display_name = COALESCE($2, display_name), role = COALESCE($3, role), active = COALESCE($4, active)
		WHERE id = $1 RETURNING `+userCols, id, p.DisplayName, p.Role, p.Active))
	if err != nil {
		return u, err
	}
	if deactivating || (p.Role != nil && *p.Role != before.Role) {
		if _, err := q.Exec(ctx, `UPDATE admin_sessions SET revoked_at = now() WHERE admin_user_id = $1 AND revoked_at IS NULL`, id); err != nil {
			return u, err
		}
	}
	return u, Audit(ctx, q, a, "admin_user.update", "admin_user", &id,
		map[string]any{"role": before.Role, "active": before.Active, "display_name": before.DisplayName},
		map[string]any{"role": u.Role, "active": u.Active, "display_name": u.DisplayName})
}

// SetPasswordByID resets an admin's password by id (see SetPassword).
func SetPasswordByID(ctx context.Context, q DBTX, a Actor, id, password string) error {
	var email string
	if err := q.QueryRow(ctx, `SELECT email::text FROM admin_users WHERE id = $1`, id).Scan(&email); err != nil {
		return err
	}
	return SetPassword(ctx, q, a, email, password)
}
