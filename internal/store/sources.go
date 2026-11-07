package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// DataSource mirrors data_sources.
type DataSource struct {
	ID                   string         `json:"id"`
	SourceType           string         `json:"source_type"`
	Provider             string         `json:"provider"`
	SourceURL            *string        `json:"source_url"`
	LicenseReference     *string        `json:"license_reference"`
	CollectedAt          time.Time      `json:"collected_at"`
	AllowedInternalUse   bool           `json:"allowed_internal_use"`
	AllowedPublicDisplay bool           `json:"allowed_public_display"`
	AllowedCommercialAPI bool           `json:"allowed_commercial_api"`
	RetentionExpiresAt   *time.Time     `json:"retention_expires_at"`
	Restrictions         map[string]any `json:"restrictions"`
	Notes                *string        `json:"notes"`
	CreatedAt            time.Time      `json:"created_at"`
}

const sourceCols = `id, source_type, provider, source_url, license_reference, collected_at, allowed_internal_use,
	allowed_public_display, allowed_commercial_api, retention_expires_at, restrictions, notes, created_at`

func scanSource(row pgx.Row) (DataSource, error) {
	var d DataSource
	err := row.Scan(&d.ID, &d.SourceType, &d.Provider, &d.SourceURL, &d.LicenseReference, &d.CollectedAt,
		&d.AllowedInternalUse, &d.AllowedPublicDisplay, &d.AllowedCommercialAPI, &d.RetentionExpiresAt,
		&d.Restrictions, &d.Notes, &d.CreatedAt)
	return d, err
}

// SourceInput creates or updates a data source. Nil pointers mean "unchanged" on update.
type SourceInput struct {
	SourceType           string     `json:"source_type" validate:"omitempty,oneof=self_submission authorized_import official_site licensed_feed permitted_api_lookup manual_verification"`
	Provider             string     `json:"provider" validate:"omitempty,max=200"`
	SourceURL            *string    `json:"source_url" validate:"omitempty,url,max=500"`
	LicenseReference     *string    `json:"license_reference" validate:"omitempty,max=500"`
	AllowedInternalUse   *bool      `json:"allowed_internal_use"`
	AllowedPublicDisplay *bool      `json:"allowed_public_display"`
	AllowedCommercialAPI *bool      `json:"allowed_commercial_api"`
	RetentionExpiresAt   *time.Time `json:"retention_expires_at"`
	Notes                *string    `json:"notes" validate:"omitempty,max=4000"`
}

func boolOr(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}

// CreateSource inserts a data source. Unspecified permissions default to false.
func CreateSource(ctx context.Context, q DBTX, a Actor, in SourceInput) (DataSource, error) {
	if in.SourceType == "" || in.Provider == "" {
		return DataSource{}, ErrInvalid{Msg: "source_type and provider are required",
			Fields: map[string]string{"source_type": "is required", "provider": "is required"}}
	}
	d, err := scanSource(q.QueryRow(ctx, `INSERT INTO data_sources
		(source_type, provider, source_url, license_reference, allowed_internal_use, allowed_public_display,
		 allowed_commercial_api, retention_expires_at, notes, declared_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING `+sourceCols,
		in.SourceType, in.Provider, in.SourceURL, in.LicenseReference, boolOr(in.AllowedInternalUse, false),
		boolOr(in.AllowedPublicDisplay, false), boolOr(in.AllowedCommercialAPI, false), in.RetentionExpiresAt,
		in.Notes, a.idOrNil()))
	if err != nil {
		return d, err
	}
	return d, Audit(ctx, q, a, "source.create", "data_source", &d.ID, nil, d)
}

// GetSource loads one source.
func GetSource(ctx context.Context, q DBTX, id string) (DataSource, error) {
	return scanSource(q.QueryRow(ctx, `SELECT `+sourceCols+` FROM data_sources WHERE id = $1`, id))
}

// ListSources returns sources newest first.
func ListSources(ctx context.Context, q DBTX, limit, offset int) ([]DataSource, int, error) {
	var total int
	if err := q.QueryRow(ctx, `SELECT count(*) FROM data_sources`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := q.Query(ctx, `SELECT `+sourceCols+` FROM data_sources ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []DataSource
	for rows.Next() {
		d, err := scanSource(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, d)
	}
	return out, total, rows.Err()
}

// UpdateSource changes a source. If public display is revoked, every published company that
// no longer satisfies the publication policy is unpublished in the same transaction.
func (s *Store) UpdateSource(ctx context.Context, a Actor, id string, in SourceInput) (DataSource, []string, error) {
	var out DataSource
	var unpublished []string
	err := s.InTx(ctx, func(tx pgx.Tx) error {
		before, err := scanSource(tx.QueryRow(ctx, `SELECT `+sourceCols+` FROM data_sources WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return err
		}
		out, err = scanSource(tx.QueryRow(ctx, `UPDATE data_sources SET
			source_type = COALESCE(NULLIF($2, ''), source_type),
			provider = COALESCE(NULLIF($3, ''), provider),
			source_url = COALESCE($4, source_url),
			license_reference = COALESCE($5, license_reference),
			allowed_internal_use = COALESCE($6, allowed_internal_use),
			allowed_public_display = COALESCE($7, allowed_public_display),
			allowed_commercial_api = COALESCE($8, allowed_commercial_api),
			retention_expires_at = COALESCE($9, retention_expires_at),
			notes = COALESCE($10, notes)
			WHERE id = $1 RETURNING `+sourceCols,
			id, in.SourceType, in.Provider, in.SourceURL, in.LicenseReference, in.AllowedInternalUse,
			in.AllowedPublicDisplay, in.AllowedCommercialAPI, in.RetentionExpiresAt, in.Notes))
		if err != nil {
			return err
		}
		if err := Audit(ctx, tx, a, "source.update", "data_source", &id, before, out); err != nil {
			return err
		}
		if before.AllowedPublicDisplay && !out.AllowedPublicDisplay {
			unpublished, err = UnpublishNonCompliant(ctx, tx, a, "source public display revoked")
		}
		return err
	})
	return out, unpublished, err
}
