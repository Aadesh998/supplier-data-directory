package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// DiscoveryLead stores only a provider identifier plus our own notes.
type DiscoveryLead struct {
	ID              string    `json:"id"`
	Provider        string    `json:"provider"`
	ProviderPlaceID string    `json:"provider_place_id"`
	SearchQuery     *string   `json:"search_query"`
	Status          string    `json:"status"`
	Notes           *string   `json:"notes"`
	CreatedAt       time.Time `json:"created_at"`
}

const leadCols = `id, provider, provider_place_id, search_query, status, notes, created_at`

func scanLead(r pgx.Row) (DiscoveryLead, error) {
	var l DiscoveryLead
	return l, r.Scan(&l.ID, &l.Provider, &l.ProviderPlaceID, &l.SearchQuery, &l.Status, &l.Notes, &l.CreatedAt)
}

// SaveLead records a place id as a lead (idempotent).
func SaveLead(ctx context.Context, q DBTX, a Actor, provider, placeID, query, notes string) (DiscoveryLead, error) {
	l, err := scanLead(q.QueryRow(ctx, `INSERT INTO discovery_leads (provider, provider_place_id, search_query, notes, created_by)
		VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),$5)
		ON CONFLICT (provider, provider_place_id) DO UPDATE SET updated_at = now() RETURNING `+leadCols, provider, placeID, query, notes, a.ID))
	if err != nil {
		return l, err
	}
	return l, Audit(ctx, q, a, "discovery.lead_save", "discovery_lead", &l.ID, nil, map[string]any{"provider": provider})
}

// ListLeads lists leads newest first.
func ListLeads(ctx context.Context, q DBTX, limit, offset int) ([]DiscoveryLead, int, error) {
	var total int
	if err := q.QueryRow(ctx, `SELECT count(*) FROM discovery_leads`).Scan(&total); err != nil {
		return nil, 0, err
	}
	out, err := collect(ctx, q, `SELECT `+leadCols+` FROM discovery_leads ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		func(r pgx.Rows) (DiscoveryLead, error) { return scanLead(r) }, limit, offset)
	return out, total, err
}

// UpdateLead changes a lead's status/notes.
func UpdateLead(ctx context.Context, q DBTX, a Actor, id string, status, notes *string) (DiscoveryLead, error) {
	l, err := scanLead(q.QueryRow(ctx, `UPDATE discovery_leads SET status = COALESCE($2, status), notes = COALESCE($3, notes)
		WHERE id = $1 RETURNING `+leadCols, id, status, notes))
	if err != nil {
		return l, err
	}
	return l, Audit(ctx, q, a, "discovery.lead_update", "discovery_lead", &id, nil, map[string]any{"status": status})
}
