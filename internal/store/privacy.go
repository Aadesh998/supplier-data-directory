package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// PrivacyRequestInput is a public correction/removal request.
type PrivacyRequestInput struct {
	RequestType      string `json:"request_type" validate:"required,oneof=correction removal withdraw_public_listing withdraw_commercial_api access other"`
	RequesterName    string `json:"requester_name" validate:"required,max=200"`
	Email            string `json:"email" validate:"required,email,max=254"`
	CompanyReference string `json:"company_reference,omitempty" validate:"max=300"`
	Details          string `json:"details,omitempty" validate:"max=5000"`
	WebsiteConfirm   string `json:"website_confirm,omitempty"`
}

// PrivacyRequest is the admin view.
type PrivacyRequest struct {
	ID               string     `json:"id"`
	RequestType      string     `json:"request_type"`
	Requester        string     `json:"requester"`
	Email            string     `json:"email"`
	CompanyID        *string    `json:"company_id"`
	CompanyReference *string    `json:"company_reference"`
	Details          *string    `json:"details"`
	Status           string     `json:"status"`
	ReceivedAt       time.Time  `json:"received_at"`
	ResolvedAt       *time.Time `json:"resolved_at"`
	ResolutionNotes  *string    `json:"resolution_notes"`
}

// CreatePrivacyRequest stores a request, linking it to a company when the reference is an exact slug.
func (s *Store) CreatePrivacyRequest(ctx context.Context, in PrivacyRequestInput, requestID string) (string, error) {
	enc, err := s.Box.Encrypt(in.Email)
	if err != nil {
		return "", err
	}
	var id string
	err = s.InTx(ctx, func(tx pgx.Tx) error {
		var companyID *string
		if in.CompanyReference != "" {
			ref := in.CompanyReference
			if i := lastSlash(ref); i >= 0 {
				ref = ref[i+1:]
			}
			_ = tx.QueryRow(ctx, `SELECT id FROM companies WHERE slug = $1`, ref).Scan(&companyID)
		}
		if err := tx.QueryRow(ctx, `INSERT INTO deletion_requests (request_type, requester, requester_email_encrypted,
			requester_email_hash, company_id, company_reference, details) VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,'')) RETURNING id`,
			in.RequestType, in.RequesterName, enc, s.Box.Hash(in.Email), companyID, in.CompanyReference, in.Details).Scan(&id); err != nil {
			return err
		}
		return Audit(ctx, tx, PublicActor(requestID), "privacy_request.create", "deletion_request", &id, nil,
			map[string]any{"request_type": in.RequestType, "company_id": companyID})
	})
	return id, err
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}

// GetPrivacyRequest loads one request with decrypted email.
func (s *Store) GetPrivacyRequest(ctx context.Context, q DBTX, id string) (PrivacyRequest, error) {
	var p PrivacyRequest
	var enc []byte
	err := q.QueryRow(ctx, `SELECT id, request_type, requester, requester_email_encrypted, company_id, company_reference, details,
		status, received_at, resolved_at, resolution_notes FROM deletion_requests WHERE id = $1`, id).
		Scan(&p.ID, &p.RequestType, &p.Requester, &enc, &p.CompanyID, &p.CompanyReference, &p.Details, &p.Status, &p.ReceivedAt,
			&p.ResolvedAt, &p.ResolutionNotes)
	if err != nil {
		return p, err
	}
	if p.Email, err = s.Box.Decrypt(enc); err != nil {
		p.Email = "[unreadable]"
	}
	return p, nil
}

// ListPrivacyRequests lists requests, oldest open first.
func (s *Store) ListPrivacyRequests(ctx context.Context, status string, limit, offset int) ([]PrivacyRequest, int, error) {
	var total int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM deletion_requests WHERE ($1 = '' OR status = $1)`, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	ids, err := collect(ctx, s.Pool, `SELECT id FROM deletion_requests WHERE ($1 = '' OR status = $1)
		ORDER BY (status IN ('received','in_progress')) DESC, received_at LIMIT $2 OFFSET $3`, func(r pgx.Rows) (string, error) {
		var x string
		return x, r.Scan(&x)
	}, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	out := make([]PrivacyRequest, 0, len(ids))
	for _, id := range ids {
		p, err := s.GetPrivacyRequest(ctx, s.Pool, id)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, nil
}

// ResolvePrivacyInput resolves a request, optionally applying an action to a company.
type ResolvePrivacyInput struct {
	Status          string  `json:"status" validate:"required,oneof=in_progress resolved rejected"`
	CompanyID       *string `json:"company_id" validate:"omitempty,uuid"`
	Action          string  `json:"action" validate:"omitempty,oneof=none unpublish withdraw_public withdraw_commercial erase_company"`
	ResolutionNotes string  `json:"resolution_notes" validate:"max=4000"`
}

// ResolvePrivacyRequest updates a request and applies the requested action atomically.
func (s *Store) ResolvePrivacyRequest(ctx context.Context, a Actor, id string, in ResolvePrivacyInput) (PrivacyRequest, error) {
	err := s.InTx(ctx, func(tx pgx.Tx) error {
		before, err := s.GetPrivacyRequest(ctx, tx, id)
		if err != nil {
			return err
		}
		companyID := before.CompanyID
		if in.CompanyID != nil {
			companyID = in.CompanyID
		}
		if in.Action != "" && in.Action != "none" {
			if companyID == nil {
				return ErrInvalid{Msg: "company required", Fields: map[string]string{"company_id": "link the request to a company to apply an action"}}
			}
			if err := applyPrivacyAction(ctx, tx, a, *companyID, in.Action); err != nil {
				return err
			}
		}
		if in.Action == "erase_company" {
			companyID = nil
		}
		if _, err := tx.Exec(ctx, `UPDATE deletion_requests SET status = $2, company_id = $3, resolution_notes = NULLIF($4, ''),
			acknowledged_at = COALESCE(acknowledged_at, now()),
			resolved_at = CASE WHEN $2 IN ('resolved', 'rejected') THEN now() ELSE NULL END,
			resolved_by = CASE WHEN $2 IN ('resolved', 'rejected') THEN $5::uuid ELSE NULL END WHERE id = $1`,
			id, in.Status, companyID, in.ResolutionNotes, a.ID); err != nil {
			return err
		}
		return Audit(ctx, tx, a, "privacy_request.resolve", "deletion_request", &id, map[string]any{"status": before.Status},
			map[string]any{"status": in.Status, "action": in.Action})
	})
	if err != nil {
		return PrivacyRequest{}, err
	}
	return s.GetPrivacyRequest(ctx, s.Pool, id)
}

func applyPrivacyAction(ctx context.Context, tx DBTX, a Actor, companyID, action string) error {
	switch action {
	case "unpublish":
		return unpublishTx(ctx, tx, a, companyID, "privacy request")
	case "withdraw_public":
		if err := unpublishTx(ctx, tx, a, companyID, "public listing permission withdrawn"); err != nil {
			return err
		}
		for _, sql := range []string{
			`UPDATE companies SET public_listing_withdrawn_at = COALESCE(public_listing_withdrawn_at, now()) WHERE id = $1`,
			`UPDATE contacts SET is_public = false WHERE company_id = $1`,
			`UPDATE supplier_submissions SET public_listing_withdrawn_at = COALESCE(public_listing_withdrawn_at, now()) WHERE company_id = $1`,
		} {
			if _, err := tx.Exec(ctx, sql, companyID); err != nil {
				return err
			}
		}
		return Audit(ctx, tx, a, "company.withdraw_public", "company", &companyID, nil, nil)
	case "withdraw_commercial":
		for _, sql := range []string{
			`UPDATE companies SET commercial_api_withdrawn_at = COALESCE(commercial_api_withdrawn_at, now()) WHERE id = $1`,
			`UPDATE supplier_submissions SET commercial_api_withdrawn_at = COALESCE(commercial_api_withdrawn_at, now()) WHERE company_id = $1`,
		} {
			if _, err := tx.Exec(ctx, sql, companyID); err != nil {
				return err
			}
		}
		return Audit(ctx, tx, a, "company.withdraw_commercial", "company", &companyID, nil, nil)
	case "erase_company":
		return EraseCompany(ctx, tx, a, companyID)
	}
	return nil
}

// EraseCompany permanently deletes a company and its dependent records and anonymises linked
// submissions. The audit log keeps only the fact of erasure (ids, no content).
func EraseCompany(ctx context.Context, tx DBTX, a Actor, companyID string) error {
	for _, sql := range []string{
		`UPDATE companies SET merged_into_id = NULL WHERE merged_into_id = $1`,
		`UPDATE supplier_submissions SET payload = '{"erased": true}'::jsonb, submitter_email_encrypted = '\x00'::bytea,
		   submitter_email_hash = 'erased', submitter_ip_hash = NULL, status = 'withdrawn', review_notes = NULL WHERE company_id = $1`,
		`UPDATE companies SET publication_status = 'archived' WHERE id = $1 AND publication_status = 'published'`,
	} {
		if _, err := tx.Exec(ctx, sql, companyID); err != nil {
			return err
		}
	}
	tag, err := tx.Exec(ctx, `DELETE FROM companies WHERE id = $1`, companyID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return Audit(ctx, tx, a, "company.erase", "company", &companyID, nil, map[string]any{"erased": true})
}
