// Package httpx contains HTTP helpers shared by all handlers: the error envelope,
// JSON encoding/decoding, validation, pagination and middleware.
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Error is an API error rendered as {"error":{code,message,request_id,details}}.
type Error struct {
	Status  int
	Code    string
	Message string
	Details map[string]any
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// E constructs an API error.
func E(status int, code, msg string) *Error { return &Error{Status: status, Code: code, Message: msg} }

// WithDetails attaches details to an error.
func (e *Error) WithDetails(d map[string]any) *Error { e.Details = d; return e }

var (
	ErrNotFound     = E(http.StatusNotFound, "not_found", "resource not found")
	ErrUnauthorized = E(http.StatusUnauthorized, "unauthorized", "authentication required")
	ErrForbidden    = E(http.StatusForbidden, "forbidden", "your role does not permit this action")
)

// FieldErrors builds a validation_failed error.
func FieldErrors(fields map[string]string) *Error {
	return E(http.StatusUnprocessableEntity, "validation_failed", "some fields are invalid").
		WithDetails(map[string]any{"fields": fields})
}

type ctxKey int

const requestIDKey ctxKey = iota

// RequestID returns the request id stored by the RequestID middleware.
func RequestID(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

// WithRequestID stores a request id in the context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// JSON writes v as JSON with the given status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// WriteError renders err. Unknown errors are logged and returned as 500 without internals.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *Error
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &apiErr):
	case errors.Is(err, pgx.ErrNoRows):
		apiErr = ErrNotFound
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		apiErr = E(http.StatusConflict, "conflict", "a record with the same unique value already exists").
			WithDetails(map[string]any{"constraint": pgErr.ConstraintName})
	case errors.As(err, &pgErr) && (pgErr.Code == "23514" || pgErr.Code == "23503" || pgErr.Code == "22P02" || pgErr.Code == "23502"):
		msg := "the change violates a data rule"
		if friendly, ok := constraintMessages[pgErr.ConstraintName]; ok {
			msg = friendly
		}
		apiErr = E(http.StatusUnprocessableEntity, "constraint_violation", msg).
			WithDetails(map[string]any{"constraint": pgErr.ConstraintName, "rule": pgErr.Message})
	case errors.Is(err, context.Canceled):
		return
	default:
		slog.ErrorContext(r.Context(), "unhandled error", "err", err, "path", r.URL.Path, "request_id", RequestID(r.Context()))
		apiErr = E(http.StatusInternalServerError, "internal_error", "an unexpected error occurred")
	}
	body := map[string]any{"code": apiErr.Code, "message": apiErr.Message, "request_id": RequestID(r.Context())}
	if apiErr.Details != nil {
		body["details"] = apiErr.Details
	}
	JSON(w, apiErr.Status, map[string]any{"error": body})
}

// Handler adapts an error-returning handler to http.HandlerFunc.
type Handler func(w http.ResponseWriter, r *http.Request) error

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h(w, r); err != nil {
		WriteError(w, r, err)
	}
}

var validate = func() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
	return v
}()

// Decode reads a JSON body (bounded by maxBytes), rejects unknown fields and validates it.
func Decode(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) error {
	if ct := r.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "application/json") {
		return E(http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		switch {
		case errors.As(err, &maxErr):
			return E(http.StatusRequestEntityTooLarge, "payload_too_large", "request body is too large")
		case errors.Is(err, io.EOF):
			return E(http.StatusBadRequest, "invalid_json", "request body is empty")
		case errors.As(err, &syntaxErr):
			return E(http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		case errors.As(err, &typeErr):
			return FieldErrors(map[string]string{typeErr.Field: "has the wrong type"})
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			return E(http.StatusBadRequest, "invalid_json", strings.TrimPrefix(err.Error(), "json: "))
		default:
			return E(http.StatusBadRequest, "invalid_json", "request body could not be parsed")
		}
	}
	if dec.More() {
		return E(http.StatusBadRequest, "invalid_json", "request body must contain a single JSON object")
	}
	return Validate(dst)
}

// Validate runs struct validation and converts failures to validation_failed.
func Validate(v any) error {
	err := validate.Struct(v)
	if err == nil {
		return nil
	}
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return err
	}
	fields := map[string]string{}
	for _, fe := range verrs {
		ns := fe.Namespace()
		if i := strings.Index(ns, "."); i >= 0 {
			ns = ns[i+1:] // drop root struct name
		}
		fields[ns] = message(fe)
	}
	return FieldErrors(fields)
}

func message(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		if fe.Kind() == reflect.Slice {
			return fmt.Sprintf("must contain at least %s item(s)", fe.Param())
		}
		return fmt.Sprintf("must be at least %s characters", fe.Param())
	case "max":
		if fe.Kind() == reflect.Slice {
			return fmt.Sprintf("must contain at most %s item(s)", fe.Param())
		}
		return fmt.Sprintf("must be at most %s characters", fe.Param())
	case "oneof":
		return "must be one of: " + fe.Param()
	case "gte", "lte":
		return fmt.Sprintf("must be %s %s", map[string]string{"gte": "≥", "lte": "≤"}[fe.Tag()], fe.Param())
	case "uuid", "uuid4":
		return "must be a UUID"
	case "len":
		return fmt.Sprintf("must be exactly %s characters", fe.Param())
	default:
		return "is invalid (" + fe.Tag() + ")"
	}
}

// PageInfo describes a page of results.
type PageInfo struct {
	Page    int  `json:"page"`
	Limit   int  `json:"limit"`
	Total   int  `json:"total"`
	HasMore bool `json:"has_more"`
}

// Paged is the list response envelope.
type Paged[T any] struct {
	Data []T      `json:"data"`
	Page PageInfo `json:"page"`
}

// NewPaged builds a list response, never returning a null data array.
func NewPaged[T any](data []T, p Pagination, total int) Paged[T] {
	if data == nil {
		data = []T{}
	}
	return Paged[T]{Data: data, Page: PageInfo{Page: p.Page, Limit: p.Limit, Total: total, HasMore: p.Offset()+len(data) < total}}
}

// Pagination holds bounded page/limit values.
type Pagination struct{ Page, Limit int }

// Offset returns the SQL offset.
func (p Pagination) Offset() int { return (p.Page - 1) * p.Limit }

// ParsePagination reads page/limit with defaults and bounds (max page depth 500 to cap scraping cost).
func ParsePagination(r *http.Request, defLimit int) Pagination {
	p := Pagination{Page: 1, Limit: defLimit}
	if v, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && v > 0 {
		p.Page = min(v, 500)
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		p.Limit = min(v, 100)
	}
	return p
}

// constraintMessages turns known database rules into messages an admin can act on.
var constraintMessages = map[string]string{
	"data_sources_public_requires_internal":     "Public display needs Internal use to be allowed too. Tick Internal use and save again.",
	"data_sources_commercial_requires_internal": "Commercial/API distribution needs Internal use to be allowed too. Tick Internal use and save again.",
}
