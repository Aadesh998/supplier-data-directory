package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bharatmfg/directory/internal/normalize"
)

// SubmissionPayload is the supplier registration form.
type SubmissionPayload struct {
	Company struct {
		DisplayName       string `json:"display_name" validate:"required,min=2,max=200"`
		LegalName         string `json:"legal_name,omitempty" validate:"max=200"`
		Description       string `json:"description,omitempty" validate:"max=2000"`
		WebsiteURL        string `json:"website_url,omitempty" validate:"max=300"`
		EstablishmentYear *int   `json:"establishment_year,omitempty" validate:"omitempty,gte=1800,lte=2100"`
	} `json:"company" validate:"required"`
	Roles      []string `json:"roles" validate:"required,min=1,max=8,dive,max=40"`
	Categories []string `json:"categories" validate:"required,min=1,max=10,dive,max=160"`
	Products   []struct {
		Name        string `json:"name" validate:"required,max=200"`
		Description string `json:"description,omitempty" validate:"max=1000"`
		Unit        string `json:"unit,omitempty" validate:"max=50"`
	} `json:"products" validate:"max=30,dive"`
	Facilities []struct {
		FacilityType string `json:"facility_type" validate:"required,oneof=factory office registered_office warehouse showroom workshop laboratory other"`
		AddressLine1 string `json:"address_line1,omitempty" validate:"max=200"`
		Locality     string `json:"locality,omitempty" validate:"max=120"`
		City         string `json:"city" validate:"required,max=120"`
		District     string `json:"district,omitempty" validate:"max=120"`
		StateCode    string `json:"state_code" validate:"required,len=2"`
		PostalCode   string `json:"postal_code,omitempty" validate:"max=10"`
	} `json:"facilities" validate:"required,min=1,max=10,dive"`
	BusinessContacts []struct {
		ContactType string `json:"contact_type" validate:"required,oneof=public_business_email switchboard sales_department"`
		Channel     string `json:"channel" validate:"required,oneof=email phone"`
		Value       string `json:"value" validate:"required,max=200"`
	} `json:"business_contacts" validate:"max=5,dive"`
	Submitter struct {
		Name        string `json:"name" validate:"required,max=120"`
		Email       string `json:"email" validate:"required,email,max=254"`
		Designation string `json:"designation,omitempty" validate:"max=120"`
		Phone       string `json:"phone,omitempty" validate:"max=30"`
	} `json:"submitter" validate:"required"`
	Consents struct {
		PrivacyNoticeAcknowledged bool `json:"privacy_notice_acknowledged"`
		PublicListing             bool `json:"public_listing"`
		CommercialAPI             bool `json:"commercial_api"`
	} `json:"consents" validate:"required"`
	PrivacyNoticeVersion string `json:"privacy_notice_version" validate:"required,max=40"`
	FormStartedAt        int64  `json:"form_started_at" validate:"required"`
	WebsiteConfirm       string `json:"website_confirm,omitempty"`
}

// ValidateSemantics checks rules struct tags cannot express, normalising values in place.
func (p *SubmissionPayload) ValidateSemantics(ctx context.Context, q DBTX, currentNotice string) error {
	fields := map[string]string{}
	if !p.Consents.PrivacyNoticeAcknowledged {
		fields["consents.privacy_notice_acknowledged"] = "you must acknowledge the privacy notice to submit"
	}
	if p.PrivacyNoticeVersion != currentNotice {
		fields["privacy_notice_version"] = "the privacy notice has changed; please reload the form and review it"
	}
	if p.Company.WebsiteURL != "" {
		if u, _, ok := normalize.WebsiteURL(p.Company.WebsiteURL); ok {
			p.Company.WebsiteURL = u
		} else {
			fields["company.website_url"] = "must be a public http(s) website address"
		}
	}
	if e, ok := normalize.Email(p.Submitter.Email); ok {
		p.Submitter.Email = e
	} else {
		fields["submitter.email"] = "must be a valid email address"
	}
	for i := range p.Facilities {
		f := &p.Facilities[i]
		if code, ok := normalize.StateCode(f.StateCode); ok {
			f.StateCode = code
		} else {
			fields[fmt.Sprintf("facilities[%d].state_code", i)] = "is not a recognised state or union territory"
		}
		if f.PostalCode != "" {
			if pin, ok := normalize.PostalCode(f.PostalCode); ok {
				f.PostalCode = pin
			} else {
				fields[fmt.Sprintf("facilities[%d].postal_code", i)] = "must be a 6-digit PIN code"
			}
		}
	}
	for i := range p.BusinessContacts {
		c := &p.BusinessContacts[i]
		switch c.Channel {
		case "email":
			if e, ok := normalize.Email(c.Value); ok {
				c.Value = e
			} else {
				fields[fmt.Sprintf("business_contacts[%d].value", i)] = "must be a valid email address"
			}
		case "phone":
			if ph, ok := normalize.Phone(c.Value); ok {
				c.Value = ph
			} else {
				fields[fmt.Sprintf("business_contacts[%d].value", i)] = "must be a phone number with STD/country code"
			}
		}
	}
	var unknownRoles, unknownCats []string
	if err := q.QueryRow(ctx, `SELECT
		COALESCE((SELECT array_agg(r) FROM unnest($1::text[]) r WHERE r NOT IN (SELECT code FROM business_roles)), '{}'),
		COALESCE((SELECT array_agg(c) FROM unnest($2::text[]) c WHERE c NOT IN (SELECT slug FROM categories WHERE active)), '{}')`,
		p.Roles, p.Categories).Scan(&unknownRoles, &unknownCats); err != nil {
		return err
	}
	if len(unknownRoles) > 0 {
		fields["roles"] = "unknown role(s): " + strings.Join(unknownRoles, ", ")
	}
	if len(unknownCats) > 0 {
		fields["categories"] = "unknown category(ies): " + strings.Join(unknownCats, ", ")
	}
	if len(fields) > 0 {
		return ErrInvalid{Msg: "invalid submission", Fields: fields}
	}
	return nil
}

// Submission is the admin list view.
type Submission struct {
	ID                   string     `json:"id"`
	Status               string     `json:"status"`
	CompanyName          string     `json:"company_name"`
	SubmittedAt          time.Time  `json:"submitted_at"`
	ReviewedAt           *time.Time `json:"reviewed_at"`
	PrivacyNoticeVersion string     `json:"privacy_notice_version"`
	Consents             struct {
		StorageAcknowledgedAt time.Time  `json:"storage_acknowledged_at"`
		PublicListingAt       *time.Time `json:"public_listing_at"`
		CommercialAPIAt       *time.Time `json:"commercial_api_at"`
	} `json:"consents"`
	CompanyID *string `json:"company_id"`
}

// SubmissionDetail adds the payload and decrypted submitter email.
type SubmissionDetail struct {
	Submission
	Payload        json.RawMessage `json:"payload"`
	SubmitterEmail string          `json:"submitter_email"`
	ReviewNotes    *string         `json:"review_notes"`
}

const submissionCols = `id, status, payload->'company'->>'display_name', submitted_at, reviewed_at, privacy_notice_version,
	storage_consent_at, public_listing_permission_at, future_commercial_api_permission_at, company_id`

func scanSubmission(r pgx.Row) (Submission, error) {
	var s Submission
	err := r.Scan(&s.ID, &s.Status, &s.CompanyName, &s.SubmittedAt, &s.ReviewedAt, &s.PrivacyNoticeVersion,
		&s.Consents.StorageAcknowledgedAt, &s.Consents.PublicListingAt, &s.Consents.CommercialAPIAt, &s.CompanyID)
	return s, err
}

// CreateSubmission stores a validated submission. With an idempotency key, a repeat request
// returns the original submission id.
func (s *Store) CreateSubmission(ctx context.Context, p SubmissionPayload, idempotencyKey, ipHash, requestID string) (string, bool, error) {
	if idempotencyKey != "" {
		var existing string
		err := s.Pool.QueryRow(ctx, `SELECT id FROM supplier_submissions WHERE idempotency_key = $1`, idempotencyKey).Scan(&existing)
		if err == nil {
			return existing, true, nil
		}
		if !IsNotFound(err) {
			return "", false, err
		}
	}
	email := p.Submitter.Email
	enc, err := s.Box.Encrypt(email)
	if err != nil {
		return "", false, err
	}
	// Submitter personal data lives only in the encrypted column, not in the JSON payload.
	stored := p
	stored.Submitter.Email = ""
	stored.WebsiteConfirm = ""
	payload, err := json.Marshal(stored)
	if err != nil {
		return "", false, err
	}
	now := time.Now()
	var pub, com *time.Time
	if p.Consents.PublicListing {
		pub = &now
	}
	if p.Consents.CommercialAPI {
		com = &now
	}
	var id string
	err = s.InTx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `INSERT INTO supplier_submissions (payload, submitter_email_encrypted, submitter_email_hash,
			submitter_ip_hash, privacy_notice_version, storage_consent_at, public_listing_permission_at,
			future_commercial_api_permission_at, idempotency_key)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,'')) RETURNING id`,
			payload, enc, s.Box.Hash(email), ipHash, p.PrivacyNoticeVersion, now, pub, com, idempotencyKey).Scan(&id)
		if err != nil {
			return err
		}
		return Audit(ctx, tx, PublicActor(requestID), "submission.create", "supplier_submission", &id, nil,
			map[string]any{"public_listing": p.Consents.PublicListing, "commercial_api": p.Consents.CommercialAPI})
	})
	return id, false, err
}

// CountRecentSubmissions counts submissions from the same email hash in the window (abuse control).
func (s *Store) CountRecentSubmissions(ctx context.Context, email string, window time.Duration) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM supplier_submissions WHERE submitter_email_hash = $1 AND submitted_at > $2`,
		s.Box.Hash(email), time.Now().Add(-window)).Scan(&n)
	return n, err
}

// ListSubmissions lists submissions, optionally by status.
func ListSubmissions(ctx context.Context, q DBTX, status string, limit, offset int) ([]Submission, int, error) {
	var total int
	if err := q.QueryRow(ctx, `SELECT count(*) FROM supplier_submissions WHERE ($1 = '' OR status = $1)`, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	out, err := collect(ctx, q, `SELECT `+submissionCols+` FROM supplier_submissions WHERE ($1 = '' OR status = $1)
		ORDER BY submitted_at DESC LIMIT $2 OFFSET $3`, func(r pgx.Rows) (Submission, error) { return scanSubmission(r) }, status, limit, offset)
	return out, total, err
}

// GetSubmission returns a submission with decrypted submitter email; access is audited.
func (s *Store) GetSubmission(ctx context.Context, a Actor, id string) (SubmissionDetail, error) {
	var d SubmissionDetail
	var enc []byte
	var payload []byte
	row := s.Pool.QueryRow(ctx, `SELECT `+submissionCols+`, payload, submitter_email_encrypted, review_notes
		FROM supplier_submissions WHERE id = $1`, id)
	err := row.Scan(&d.ID, &d.Status, &d.CompanyName, &d.SubmittedAt, &d.ReviewedAt, &d.PrivacyNoticeVersion,
		&d.Consents.StorageAcknowledgedAt, &d.Consents.PublicListingAt, &d.Consents.CommercialAPIAt, &d.CompanyID,
		&payload, &enc, &d.ReviewNotes)
	if err != nil {
		return d, err
	}
	d.Payload = payload
	if d.SubmitterEmail, err = s.Box.Decrypt(enc); err != nil {
		d.SubmitterEmail = "[unreadable]"
	}
	if err := Audit(ctx, s.Pool, a, "submission.view_personal_data", "supplier_submission", &id, nil, nil); err != nil {
		return d, err
	}
	return d, nil
}

// SubmitterEmail decrypts the submitter email for notifications.
func (s *Store) SubmitterEmail(ctx context.Context, id string) (string, string, error) {
	var enc []byte
	var name string
	if err := s.Pool.QueryRow(ctx, `SELECT submitter_email_encrypted, payload->'company'->>'display_name'
		FROM supplier_submissions WHERE id = $1`, id).Scan(&enc, &name); err != nil {
		return "", "", err
	}
	email, err := s.Box.Decrypt(enc)
	return email, name, err
}

// ApproveSubmission converts a submission into an approved_private company. It never publishes.
// Business fields receive provenance from a self_submission source whose public/commercial flags
// mirror the supplier's separate authorisations.
func (s *Store) ApproveSubmission(ctx context.Context, a Actor, id, notes string) (string, error) {
	var companyID string
	err := s.InTx(ctx, func(tx pgx.Tx) error {
		var status, notice string
		var payloadRaw []byte
		var emailEnc []byte
		var submittedAt time.Time
		var pubAt, comAt, pubWd, comWd *time.Time
		err := tx.QueryRow(ctx, `SELECT status, payload, submitter_email_encrypted, privacy_notice_version, submitted_at,
			public_listing_permission_at, future_commercial_api_permission_at, public_listing_withdrawn_at, commercial_api_withdrawn_at
			FROM supplier_submissions WHERE id = $1 FOR UPDATE`, id).
			Scan(&status, &payloadRaw, &emailEnc, &notice, &submittedAt, &pubAt, &comAt, &pubWd, &comWd)
		if err != nil {
			return err
		}
		if status != "submitted" && status != "under_review" && status != "needs_changes" {
			return ErrConflict{Msg: "submission has already been " + status}
		}
		var p SubmissionPayload
		if err := json.Unmarshal(payloadRaw, &p); err != nil {
			return fmt.Errorf("decode payload: %w", err)
		}
		public := pubAt != nil && pubWd == nil
		commercial := comAt != nil && comWd == nil
		var sourceID string
		if err := tx.QueryRow(ctx, `INSERT INTO data_sources (source_type, provider, license_reference, collected_at,
			allowed_internal_use, allowed_public_display, allowed_commercial_api, notes, declared_by, restrictions)
			VALUES ('self_submission', 'supplier-registration-form', $1, $2, true, $3, $4, $5, $6, $7) RETURNING id`,
			"submission:"+id+"; privacy-notice:"+notice, submittedAt, public, commercial,
			"Created on approval of a supplier self-registration.", a.ID,
			map[string]any{"submission_id": id, "commercial_fields": []string{"display_name", "legal_name", "description",
				"website_url", "categories", "roles", "products", "city", "state_code", "business_contacts"}}).Scan(&sourceID); err != nil {
			return err
		}
		catIDs, err := collect(ctx, tx, `SELECT id FROM categories WHERE slug = ANY($1)`, func(r pgx.Rows) (string, error) {
			var x string
			return x, r.Scan(&x)
		}, p.Categories)
		if err != nil {
			return err
		}
		in := CompanyInput{SourceID: &sourceID, DisplayName: &p.Company.DisplayName, Roles: &p.Roles, CategoryIDs: &catIDs}
		if p.Company.LegalName != "" {
			in.LegalName = &p.Company.LegalName
		}
		if p.Company.Description != "" {
			in.Description = &p.Company.Description
		}
		if p.Company.WebsiteURL != "" {
			in.WebsiteURL = &p.Company.WebsiteURL
		}
		in.EstablishmentYear = p.Company.EstablishmentYear
		if len(catIDs) > 0 {
			in.PrimaryCategoryID = &catIDs[0]
		}
		companyID, err = createCompanyTx(ctx, tx, a, in, "approved_private")
		if err != nil {
			return err
		}
		for i, f := range p.Facilities {
			fi := FacilityInput{SourceID: sourceID, FacilityType: f.FacilityType, IsPrimary: i == 0,
				AddressLine1: strOrNil(f.AddressLine1), Locality: strOrNil(f.Locality), City: strOrNil(f.City),
				District: strOrNil(f.District), StateCode: strOrNil(f.StateCode), PostalCode: strOrNil(f.PostalCode)}
			if _, err := insertFacility(ctx, tx, companyID, fi); err != nil {
				return err
			}
		}
		for _, pr := range p.Products {
			pi := ProductInput{SourceID: sourceID, Name: pr.Name, Description: strOrNil(pr.Description), Unit: strOrNil(pr.Unit), CategoryID: in.PrimaryCategoryID}
			if _, err := insertProduct(ctx, tx, companyID, pi); err != nil {
				return err
			}
		}
		basis := "supplier-submission:" + id + "; privacy-notice:" + notice
		for _, c := range p.BusinessContacts {
			var cid string
			if err := tx.QueryRow(ctx, `INSERT INTO contacts (company_id, contact_type, channel, value, value_hash, is_public,
				lawful_basis_or_permission_ref, source_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
				companyID, c.ContactType, c.Channel, c.Value, s.Box.Hash(c.Value), public, basis, sourceID).Scan(&cid); err != nil {
				return err
			}
			if err := RecordProvenance(ctx, tx, companyID, "contact", cid, sourceID, "value"); err != nil {
				return err
			}
		}
		// The submitter's own email is kept as an encrypted, never-public individual contact.
		email, err := s.Box.Decrypt(emailEnc)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO contacts (company_id, contact_type, channel, label, value_encrypted, value_hash,
			is_public, lawful_basis_or_permission_ref, source_id) VALUES ($1,'individual','email',$2,$3,$4,false,$5,$6)`,
			companyID, "Submitter", emailEnc, s.Box.Hash(email), basis, sourceID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE supplier_submissions SET status = 'approved', reviewed_at = now(), reviewer_id = $2,
			review_notes = NULLIF($3, ''), company_id = $4, source_id = $5 WHERE id = $1`, id, a.ID, notes, companyID, sourceID); err != nil {
			return err
		}
		if _, err := FindDuplicatesFor(ctx, tx, companyID); err != nil {
			return err
		}
		return Audit(ctx, tx, a, "submission.approve", "supplier_submission", &id, map[string]any{"status": status},
			map[string]any{"status": "approved", "company_id": companyID, "source_id": sourceID})
	})
	return companyID, err
}

// RejectSubmission marks a submission rejected, needs_changes or spam.
func (s *Store) RejectSubmission(ctx context.Context, a Actor, id, status, reason string) error {
	return s.InTx(ctx, func(tx pgx.Tx) error {
		var current string
		if err := tx.QueryRow(ctx, `SELECT status FROM supplier_submissions WHERE id = $1 FOR UPDATE`, id).Scan(&current); err != nil {
			return err
		}
		if current == "approved" || current == "withdrawn" {
			return ErrConflict{Msg: "submission has already been " + current}
		}
		if _, err := tx.Exec(ctx, `UPDATE supplier_submissions SET status = $2, reviewed_at = now(), reviewer_id = $3, review_notes = $4
			WHERE id = $1`, id, status, a.ID, reason); err != nil {
			return err
		}
		return Audit(ctx, tx, a, "submission."+status, "supplier_submission", &id, map[string]any{"status": current},
			map[string]any{"status": status, "reason": reason})
	})
}

// GetSubmissionSummary returns the list view of one submission.
func GetSubmissionSummary(ctx context.Context, q DBTX, id string) (Submission, error) {
	return scanSubmission(q.QueryRow(ctx, `SELECT `+submissionCols+` FROM supplier_submissions WHERE id = $1`, id))
}
