package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bharatmfg/directory/internal/testutil"
)

func expectSQLState(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected SQLSTATE %s, got success", code)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("expected postgres error %s, got %v", code, err)
	}
	if pgErr.Code != code {
		t.Fatalf("expected SQLSTATE %s, got %s (%s)", code, pgErr.Code, pgErr.Message)
	}
}

const (
	checkViolation   = "23514"
	uniqueViolation  = "23505"
	insufficientPriv = "42501"
)

func newCompany(t *testing.T, pool *pgxpool.Pool, slug string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `INSERT INTO companies (display_name, slug, normalized_name)
		VALUES ($1, $2, $1) RETURNING id`, "Test "+slug, slug).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func newSource(t *testing.T, pool *pgxpool.Pool, internal, public, commercial bool) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `INSERT INTO data_sources
		(source_type, provider, allowed_internal_use, allowed_public_display, allowed_commercial_api)
		VALUES ('self_submission', 'test', $1, $2, $3) RETURNING id`, internal, public, commercial).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestDataSourcePermissionsDefaultFalse(t *testing.T) {
	pool := testutil.NewDB(t)
	var internal, public, commercial bool
	err := pool.QueryRow(context.Background(), `INSERT INTO data_sources (source_type, provider)
		VALUES ('authorized_import', 'legacy-csv')
		RETURNING allowed_internal_use, allowed_public_display, allowed_commercial_api`).Scan(&internal, &public, &commercial)
	if err != nil {
		t.Fatal(err)
	}
	if internal || public || commercial {
		t.Fatalf("permission flags must default to false, got %v %v %v", internal, public, commercial)
	}
}

func TestDataSourcePermissionImplications(t *testing.T) {
	pool := testutil.NewDB(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO data_sources (source_type, provider, allowed_public_display) VALUES ('official_site','x', true)`)
	expectSQLState(t, err, checkViolation)
	_, err = pool.Exec(ctx, `INSERT INTO data_sources (source_type, provider, allowed_commercial_api) VALUES ('official_site','x', true)`)
	expectSQLState(t, err, checkViolation)
	_, err = pool.Exec(ctx, `INSERT INTO data_sources (source_type, provider, allowed_internal_use, allowed_public_display)
		VALUES ('permitted_api_lookup','google_places', true, true)`)
	expectSQLState(t, err, checkViolation)
}

func TestPublicationRequiresPublicPermission(t *testing.T) {
	pool := testutil.NewDB(t)
	ctx := context.Background()
	cid := newCompany(t, pool, "gate-test")

	_, err := pool.Exec(ctx, `UPDATE companies SET publication_status='published', published_at=now() WHERE id=$1`, cid)
	expectSQLState(t, err, checkViolation)

	// Internal-only source is still not enough.
	private := newSource(t, pool, true, false, false)
	if _, err := pool.Exec(ctx, `INSERT INTO field_provenance (company_id, entity_type, entity_id, field_name, source_id, permission_status)
		VALUES ($1,'company',$1,'display_name',$2,'permitted')`, cid, private); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `UPDATE companies SET publication_status='published', published_at=now() WHERE id=$1`, cid)
	expectSQLState(t, err, checkViolation)

	public := newSource(t, pool, true, true, false)
	if _, err := pool.Exec(ctx, `INSERT INTO field_provenance (company_id, entity_type, entity_id, field_name, source_id, permission_status)
		VALUES ($1,'company',$1,'display_name',$2,'permitted')`, cid, public); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE companies SET publication_status='published', published_at=now() WHERE id=$1`, cid); err != nil {
		t.Fatalf("publish with permission: %v", err)
	}

	// Soft-deleted companies cannot remain published.
	_, err = pool.Exec(ctx, `UPDATE companies SET deleted_at=now() WHERE id=$1`, cid)
	expectSQLState(t, err, checkViolation)
}

func TestIndividualContactsProtected(t *testing.T) {
	pool := testutil.NewDB(t)
	ctx := context.Background()
	cid := newCompany(t, pool, "contact-test")
	_, err := pool.Exec(ctx, `INSERT INTO contacts (company_id, contact_type, channel, value, value_hash)
		VALUES ($1,'individual','phone','+91 98000 00000','h')`, cid)
	expectSQLState(t, err, checkViolation)
	_, err = pool.Exec(ctx, `INSERT INTO contacts (company_id, contact_type, channel, value_encrypted, value_hash, is_public, lawful_basis_or_permission_ref)
		VALUES ($1,'individual','phone','\x00','h', true, 'consent')`, cid)
	expectSQLState(t, err, checkViolation)
	_, err = pool.Exec(ctx, `INSERT INTO contacts (company_id, contact_type, channel, value, value_hash, is_public)
		VALUES ($1,'switchboard','phone','+91 11 0000 0000','h', true)`, cid)
	expectSQLState(t, err, checkViolation)
	if _, err := pool.Exec(ctx, `INSERT INTO contacts (company_id, contact_type, channel, value_encrypted, value_hash)
		VALUES ($1,'individual','email','\x01','h')`, cid); err != nil {
		t.Fatalf("encrypted individual contact should be allowed: %v", err)
	}
}

func TestCertificationNeverDefaultsVerified(t *testing.T) {
	pool := testutil.NewDB(t)
	ctx := context.Background()
	cid := newCompany(t, pool, "cert-test")
	var status string
	if err := pool.QueryRow(ctx, `INSERT INTO certifications (company_id, certification_type) VALUES ($1,'ISO 9001') RETURNING status`, cid).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "unverified" {
		t.Fatalf("certification default status = %s", status)
	}
	_, err := pool.Exec(ctx, `INSERT INTO certifications (company_id, certification_type, status) VALUES ($1,'ISO 9001','verified')`, cid)
	expectSQLState(t, err, checkViolation)
}

func TestCategoryCyclePrevented(t *testing.T) {
	pool := testutil.NewDB(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `UPDATE categories SET parent_id = (SELECT id FROM categories WHERE slug='hand-knotted-rugs')
		WHERE slug='textiles-and-apparel'`)
	expectSQLState(t, err, checkViolation)
	_, err = pool.Exec(ctx, `INSERT INTO categories (name, slug) VALUES ('Dup', 'tiles')`)
	expectSQLState(t, err, uniqueViolation)
}

func TestSlugAndPinValidation(t *testing.T) {
	pool := testutil.NewDB(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO companies (display_name, slug, normalized_name) VALUES ('Bad', 'Bad Slug', 'bad')`)
	expectSQLState(t, err, checkViolation)
	cid := newCompany(t, pool, "pin-test")
	_, err = pool.Exec(ctx, `INSERT INTO facilities (company_id, postal_code) VALUES ($1, '01234')`, cid)
	expectSQLState(t, err, checkViolation)
	_, err = pool.Exec(ctx, `INSERT INTO facilities (company_id, state_code, postal_code) VALUES ($1, 'MH', '411001')`, cid)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO facilities (company_id, latitude, longitude) VALUES ($1, 18.5, 73.8)`, cid)
	expectSQLState(t, err, checkViolation) // coordinates need their own source
}

func TestAuditLogAppendOnly(t *testing.T) {
	pool := testutil.NewDB(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO audit_logs (actor_type, action, entity_type) VALUES ('system','test.event','test')`); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, `UPDATE audit_logs SET action='tampered'`)
	expectSQLState(t, err, insufficientPriv)
	_, err = pool.Exec(ctx, `DELETE FROM audit_logs`)
	expectSQLState(t, err, insufficientPriv)
}

func TestAPIClientsDisabledInV1(t *testing.T) {
	pool := testutil.NewDB(t)
	_, err := pool.Exec(context.Background(), `INSERT INTO api_clients (name, status) VALUES ('acme','active')`)
	expectSQLState(t, err, checkViolation)
}

func TestSubmissionRequiresStorageAcknowledgement(t *testing.T) {
	pool := testutil.NewDB(t)
	_, err := pool.Exec(context.Background(), `INSERT INTO supplier_submissions
		(payload, submitter_email_encrypted, submitter_email_hash, privacy_notice_version)
		VALUES ('{}', '\x00', 'h', 'v1')`)
	if err == nil {
		t.Fatal("expected not-null violation for storage_consent_at")
	}
}

func TestSyntheticSeedIsPrivate(t *testing.T) {
	pool := testutil.NewDB(t)
	testutil.Seed(t, pool)
	ctx := context.Background()
	var total, published, publicFields int
	if err := pool.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE publication_status='published') FROM companies`).Scan(&total, &published); err != nil {
		t.Fatal(err)
	}
	if total != 20 || published != 0 {
		t.Fatalf("synthetic seed: total=%d published=%d, want 20/0", total, published)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public_field_permissions`).Scan(&publicFields); err != nil {
		t.Fatal(err)
	}
	if publicFields != 0 {
		t.Fatalf("synthetic seed must grant no public field permissions, got %d", publicFields)
	}
	var notSynthetic int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM companies WHERE display_name NOT LIKE '%(Synthetic)' OR website_domain NOT LIKE '%.example'`).Scan(&notSynthetic)
	if notSynthetic != 0 {
		t.Fatalf("%d seed companies are not clearly labelled synthetic", notSynthetic)
	}
	// Seed is idempotent.
	testutil.Seed(t, pool)
}
