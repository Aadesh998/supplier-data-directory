package store

import (
	"context"
	"time"
)

// Retention windows for intake data that never became a company record.
const (
	SpamRetention     = 30 * 24 * time.Hour
	RejectedRetention = 180 * 24 * time.Hour
	SessionRetention  = 30 * 24 * time.Hour
)

// MaintenanceReport summarises a maintenance run.
type MaintenanceReport struct {
	MarkedStale        int64    `json:"marked_stale"`
	Unpublished        []string `json:"unpublished"`
	SessionsPurged     int64    `json:"sessions_purged"`
	SubmissionsPurged  int64    `json:"submissions_purged"`
	DiscoveryLeadsAged int64    `json:"discovery_leads_dismissed"`
}

// RunMaintenance applies re-verification, retention and permission-expiry rules. Safe to run repeatedly.
func (s *Store) RunMaintenance(ctx context.Context) (MaintenanceReport, error) {
	var r MaintenanceReport
	var err error
	a := SystemActor()
	if r.MarkedStale, err = MarkStale(ctx, s.Pool); err != nil {
		return r, err
	}
	// Sources whose retention period has ended no longer back public fields.
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return r, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if r.Unpublished, err = UnpublishNonCompliant(ctx, tx, a, "maintenance: permission no longer valid"); err != nil {
		return r, err
	}
	if err := tx.Commit(ctx); err != nil {
		return r, err
	}
	tag, err := s.Pool.Exec(ctx, `DELETE FROM admin_sessions WHERE expires_at < $1 OR revoked_at < $1`, time.Now().Add(-SessionRetention))
	if err != nil {
		return r, err
	}
	r.SessionsPurged = tag.RowsAffected()
	tag, err = s.Pool.Exec(ctx, `DELETE FROM supplier_submissions WHERE company_id IS NULL AND
		((status = 'spam' AND submitted_at < $1) OR (status = 'rejected' AND reviewed_at < $2))`,
		time.Now().Add(-SpamRetention), time.Now().Add(-RejectedRetention))
	if err != nil {
		return r, err
	}
	r.SubmissionsPurged = tag.RowsAffected()
	if r.MarkedStale > 0 || len(r.Unpublished) > 0 || r.SubmissionsPurged > 0 {
		err = Audit(ctx, s.Pool, a, "maintenance.run", "system", nil, nil, r)
	}
	return r, err
}
