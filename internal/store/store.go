// Package store implements data access and transactional domain operations.
// All SQL is parameterised. Functions that mutate data write an audit log entry in the
// same transaction.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bharatmfg/directory/internal/secure"
)

// DBTX is satisfied by *pgxpool.Pool and pgx.Tx.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Store bundles the pool and the field encryption box.
type Store struct {
	Pool *pgxpool.Pool
	Box  *secure.Box
}

// New creates a Store.
func New(pool *pgxpool.Pool, box *secure.Box) *Store { return &Store{Pool: pool, Box: box} }

// InTx runs fn in a transaction, committing on success.
func (s *Store) InTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, s.Pool, fn)
}

// Actor identifies who is performing an action, for auditing.
type Actor struct {
	ID        string // admin user id; empty for public/system actors
	Email     string
	Role      string
	Type      string // admin | public | system
	RequestID string
}

// SystemActor is used by maintenance jobs.
func SystemActor() Actor { return Actor{Type: "system"} }

// PublicActor is used for unauthenticated submissions.
func PublicActor(requestID string) Actor { return Actor{Type: "public", RequestID: requestID} }

func (a Actor) idOrNil() *string {
	if a.ID == "" {
		return nil
	}
	return &a.ID
}

// ErrConflict is returned for state conflicts (e.g. approving an already approved submission).
type ErrConflict struct{ Msg string }

func (e ErrConflict) Error() string { return e.Msg }

// ErrInvalid is returned when input is semantically invalid; Fields maps field -> message.
type ErrInvalid struct {
	Msg    string
	Fields map[string]string
}

func (e ErrInvalid) Error() string { return e.Msg }

// ErrPolicy is returned when a permission/policy check blocks an action.
type ErrPolicy struct {
	Code    string
	Msg     string
	Reasons []string
}

func (e ErrPolicy) Error() string { return e.Msg }

// IsNotFound reports whether err is a missing-row error.
func IsNotFound(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// sensitiveKeys are stripped from audit snapshots.
var sensitiveKeys = map[string]bool{
	"password": true, "password_hash": true, "submitter_email": true, "email": true, "phone": true,
	"value": true, "value_encrypted": true, "token": true, "csrf_token": true, "submitter": true,
	"submitter_email_encrypted": true, "requester_email_encrypted": true, "api_key": true,
}

// Redact removes personal and secret fields from an audit snapshot (recursively).
func Redact(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if sensitiveKeys[strings.ToLower(k)] {
				out[k] = "[redacted]"
				continue
			}
			out[k] = Redact(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = Redact(val)
		}
		return out
	default:
		return v
	}
}

func toMap(v any) any {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out any
	_ = json.Unmarshal(b, &out)
	return Redact(out)
}

// Audit appends an audit entry. before/after are JSON-marshalled and redacted.
func Audit(ctx context.Context, q DBTX, a Actor, action, entityType string, entityID *string, before, after any) error {
	actorType := a.Type
	if actorType == "" {
		actorType = "admin"
	}
	var b, af []byte
	if m := toMap(before); m != nil {
		b, _ = json.Marshal(m)
	}
	if m := toMap(after); m != nil {
		af, _ = json.Marshal(m)
	}
	_, err := q.Exec(ctx, `INSERT INTO audit_logs (actor_id, actor_type, action, entity_type, entity_id, before, after, request_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''))`,
		a.idOrNil(), actorType, action, entityType, entityID, b, af, a.RequestID)
	if err != nil {
		return fmt.Errorf("audit %s: %w", action, err)
	}
	return nil
}

func ptr[T any](v T) *T { return &v }

// strOrNil converts "" to nil.
func strOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func maskValue(channel, v string) string {
	return secure.Mask(channel, v)
}
