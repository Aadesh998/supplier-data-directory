package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bharatmfg/directory/internal/api"
	"github.com/bharatmfg/directory/internal/config"
	"github.com/bharatmfg/directory/internal/notify"
	"github.com/bharatmfg/directory/internal/secure"
	"github.com/bharatmfg/directory/internal/store"
	"github.com/bharatmfg/directory/internal/testutil"
)

const testPassword = "correct-horse-battery"

type env struct {
	t   *testing.T
	srv *httptest.Server
	st  *store.Store
}

func newEnv(t *testing.T) *env {
	t.Helper()
	pool := testutil.NewDB(t)
	testutil.Seed(t, pool)
	box, err := secure.NewBox("dGVzdC1rZXktdGVzdC1rZXktdGVzdC1rZXktdGVzdC1rZXk=")
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(pool, box)
	cfg := config.Config{Env: "test", PublicSiteURL: "http://site.test", CORSAllowedOrigins: []string{"http://site.test"},
		SessionTTL: time.Hour, PrivacyNoticeVersion: "2026-11-v1", ReverifyInterval: 365 * 24 * time.Hour, MaxUploadBytes: 10 << 20}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httptest.NewServer(api.NewServer(cfg, st, log, notify.LogMailer{Log: log}, nil).Routes())
	t.Cleanup(srv.Close)
	for _, role := range []string{"owner", "admin", "reviewer"} {
		if _, err := store.CreateAdminUser(context.Background(), pool, store.SystemActor(), role+"@admin.test", role, testPassword, role); err != nil {
			t.Fatal(err)
		}
	}
	return &env{t: t, srv: srv, st: st}
}

type client struct {
	e    *env
	http *http.Client
	csrf string
}

func (e *env) anon() *client {
	jar, _ := cookiejar.New(nil)
	return &client{e: e, http: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (e *env) login(role string) *client {
	c := e.anon()
	var out struct {
		CSRFToken string `json:"csrf_token"`
	}
	c.must("POST", "/api/v1/admin/login", map[string]string{"email": role + "@admin.test", "password": testPassword}, 200, &out)
	c.csrf = out.CSRFToken
	return c
}

func (c *client) do(method, path string, body any, headers ...string) (int, []byte, http.Header) {
	c.e.t.Helper()
	var r io.Reader
	ct := ""
	switch b := body.(type) {
	case nil:
	case *multipartBody:
		r, ct = b.buf, b.ct
	default:
		j, _ := json.Marshal(b)
		r, ct = bytes.NewReader(j), "application/json"
	}
	req, _ := http.NewRequest(method, c.e.srv.URL+path, r)
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	if c.csrf != "" {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b, resp.Header
}

func (c *client) must(method, path string, body any, want int, out any, headers ...string) []byte {
	c.e.t.Helper()
	code, b, _ := c.do(method, path, body, headers...)
	if code != want {
		c.e.t.Fatalf("%s %s = %d, want %d: %s", method, path, code, want, b)
	}
	if out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			c.e.t.Fatalf("decode %s: %v (%s)", path, err, b)
		}
	}
	return b
}

type multipartBody struct {
	buf *bytes.Buffer
	ct  string
}

func csvUpload(filename, content string, fields map[string]string) *multipartBody {
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	fw, _ := w.CreateFormFile("file", filename)
	_, _ = fw.Write([]byte(content))
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	_ = w.Close()
	return &multipartBody{buf: buf, ct: w.FormDataContentType()}
}

func submission(name string, public, commercial bool) map[string]any {
	return map[string]any{
		"company":    map[string]any{"display_name": name, "description": "Makes test things.", "website_url": "https://" + strings.ToLower(strings.ReplaceAll(name, " ", "")) + ".example"},
		"roles":      []string{"manufacturer"},
		"categories": []string{"vitrified-tiles"},
		"products":   []map[string]any{{"name": "Glazed tile"}},
		"facilities": []map[string]any{{"facility_type": "factory", "city": "Morbi", "state_code": "GJ", "postal_code": "363641"}},
		"business_contacts": []map[string]any{{"contact_type": "sales_department", "channel": "email", "value": "sales@" +
			strings.ToLower(strings.ReplaceAll(name, " ", "")) + ".example"}},
		"submitter":              map[string]any{"name": "Priya Test", "email": "priya.private@person.example", "phone": "9876543210"},
		"consents":               map[string]any{"privacy_notice_acknowledged": true, "public_listing": public, "commercial_api": commercial},
		"privacy_notice_version": "2026-11-v1",
		"form_started_at":        time.Now().Add(-2 * time.Minute).UnixMilli(),
	}
}

func TestHealth(t *testing.T) {
	e := newEnv(t)
	c := e.anon()
	c.must("GET", "/healthz", nil, 200, nil)
	c.must("GET", "/readyz", nil, 200, nil)
	code, b, _ := c.do("GET", "/api/v1/nope", nil)
	if code != 404 || !strings.Contains(string(b), `"request_id"`) {
		t.Fatalf("error envelope missing: %d %s", code, b)
	}
}

func TestAdminRequiresAuthAndCSRF(t *testing.T) {
	e := newEnv(t)
	anon := e.anon()
	for _, p := range []string{"/api/v1/admin/me", "/api/v1/admin/companies", "/api/v1/admin/submissions", "/api/v1/admin/audit-logs", "/api/v1/admin/stats"} {
		if code, _, _ := anon.do("GET", p, nil); code != 401 {
			t.Errorf("GET %s without session = %d, want 401", p, code)
		}
	}
	anon.must("POST", "/api/v1/admin/login", map[string]string{"email": "owner@admin.test", "password": "wrong-password"}, 401, nil)

	owner := e.login("owner")
	csrf := owner.csrf
	owner.csrf = ""
	if code, b, _ := owner.do("POST", "/api/v1/admin/duplicates/scan", nil); code != 403 || !strings.Contains(string(b), "csrf_failed") {
		t.Fatalf("mutation without CSRF = %d %s", code, b)
	}
	owner.csrf = csrf
	if code, _, _ := owner.do("POST", "/api/v1/admin/duplicates/scan", nil, "Origin", "https://evil.example"); code != 403 {
		t.Fatalf("cross-origin mutation = %d, want 403", code)
	}
	owner.must("POST", "/api/v1/admin/duplicates/scan", nil, 200, nil)
	owner.must("POST", "/api/v1/admin/logout", nil, 204, nil)
	if code, _, _ := owner.do("GET", "/api/v1/admin/me", nil); code != 401 {
		t.Fatalf("session should be revoked after logout, got %d", code)
	}
}

func TestLoginLockout(t *testing.T) {
	e := newEnv(t)
	c := e.anon()
	for i := 0; i < 5; i++ {
		c.do("POST", "/api/v1/admin/login", map[string]string{"email": "admin@admin.test", "password": "nope-nope-nope"})
	}
	code, _, _ := c.do("POST", "/api/v1/admin/login", map[string]string{"email": "admin@admin.test", "password": testPassword})
	if code != 429 {
		t.Fatalf("expected lockout (429) after repeated failures, got %d", code)
	}
}

func TestRBAC(t *testing.T) {
	e := newEnv(t)
	reviewer := e.login("reviewer")
	admin := e.login("admin")
	var list struct {
		Data []struct{ ID string } `json:"data"`
	}
	reviewer.must("GET", "/api/v1/admin/companies", nil, 200, &list)
	id := list.Data[0].ID
	reviewer.must("POST", "/api/v1/admin/companies/"+id+"/publish", nil, 403, nil)
	reviewer.must("POST", "/api/v1/admin/exports", map[string]string{"scope": "internal"}, 403, nil)
	admin.must("POST", "/api/v1/admin/exports", map[string]string{"scope": "commercial"}, 403, nil)
	admin.must("GET", "/api/v1/admin/users", nil, 403, nil)
	admin.must("POST", "/api/v1/admin/sources", map[string]any{"source_type": "licensed_feed", "provider": "x", "allowed_internal_use": true, "allowed_commercial_api": true}, 403, nil)
}

func TestSubmissionToPublicationFlow(t *testing.T) {
	e := newEnv(t)
	pub := e.anon()
	var created struct{ ID string }
	pub.must("POST", "/api/v1/supplier-submissions", submission("Gujarat Glaze Works", true, false), 202, &created, "Idempotency-Key", "test-key-0001")
	// Idempotent retry returns the same submission.
	var again struct{ ID string }
	pub.must("POST", "/api/v1/supplier-submissions", submission("Gujarat Glaze Works", true, false), 202, &again, "Idempotency-Key", "test-key-0001")
	if again.ID != created.ID {
		t.Fatalf("idempotency key ignored: %s vs %s", again.ID, created.ID)
	}

	// Pending submissions are invisible publicly.
	assertNotPublic(t, pub, "gujarat-glaze-works")

	reviewer := e.login("reviewer")
	var detail struct {
		SubmitterEmail string          `json:"submitter_email"`
		Payload        json.RawMessage `json:"payload"`
	}
	reviewer.must("GET", "/api/v1/admin/submissions/"+created.ID, nil, 200, &detail)
	if detail.SubmitterEmail != "priya.private@person.example" {
		t.Fatalf("submitter email = %q", detail.SubmitterEmail)
	}
	if strings.Contains(string(detail.Payload), "priya.private") {
		t.Fatal("submitter email must not be stored in the plaintext payload")
	}
	var approved struct {
		CompanyID string `json:"company_id"`
	}
	reviewer.must("POST", "/api/v1/admin/submissions/"+created.ID+"/approve", map[string]string{"notes": "ok"}, 200, &approved)
	reviewer.must("POST", "/api/v1/admin/submissions/"+created.ID+"/approve", nil, 409, nil)

	// Approved private is still not public.
	assertNotPublic(t, pub, "gujarat-glaze-works")

	admin := e.login("admin")
	var company struct {
		PublicationStatus  string `json:"publication_status"`
		VerificationStatus string `json:"verification_status"`
		Slug               string `json:"slug"`
		Contacts           []struct {
			ContactType string `json:"contact_type"`
			Value       string `json:"value"`
			Masked      bool   `json:"masked"`
		} `json:"contacts"`
		PublishCheck struct{ Allowed bool } `json:"publish_check"`
	}
	admin.must("GET", "/api/v1/admin/companies/"+approved.CompanyID, nil, 200, &company)
	if company.PublicationStatus != "approved_private" || !company.PublishCheck.Allowed {
		t.Fatalf("unexpected company state: %+v", company)
	}
	for _, ct := range company.Contacts {
		if ct.ContactType == "individual" && (!ct.Masked || strings.Contains(ct.Value, "priya.private")) {
			t.Fatalf("individual contact not masked in admin view: %+v", ct)
		}
	}
	admin.must("POST", "/api/v1/admin/companies/"+approved.CompanyID+"/publish", nil, 200, &company)
	if company.VerificationStatus != "unverified" {
		t.Fatal("publishing must not imply verification")
	}

	// Now public — with only permitted fields, no personal data.
	body := pub.must("GET", "/api/v1/public/companies/gujarat-glaze-works", nil, 200, nil)
	for _, leak := range []string{"priya", "person.example", "9876543210", "individual", "Priya"} {
		if strings.Contains(string(body), leak) {
			t.Fatalf("public profile leaks %q: %s", leak, body)
		}
	}
	if !strings.Contains(string(body), "sales@gujaratglazeworks.example") || !strings.Contains(string(body), "Morbi") {
		t.Fatalf("public profile missing permitted fields: %s", body)
	}
	var search struct {
		Data []struct{ Slug string } `json:"data"`
	}
	pub.must("GET", "/api/v1/public/companies?category=tiles&state=gujarat", nil, 200, &search)
	if len(search.Data) != 1 || search.Data[0].Slug != "gujarat-glaze-works" {
		t.Fatalf("search (category subtree + state slug) = %+v", search.Data)
	}
	sm := pub.must("GET", "/api/v1/public/sitemap", nil, 200, nil)
	if !strings.Contains(string(sm), "gujarat-glaze-works") || strings.Contains(string(sm), "synthetic") {
		t.Fatalf("sitemap wrong: %s", sm)
	}

	// Slug change redirects.
	admin.must("PATCH", "/api/v1/admin/companies/"+approved.CompanyID, map[string]any{"slug": "gujarat-glaze"}, 200, nil)
	code, b, hdr := pub.do("GET", "/api/v1/public/companies/gujarat-glaze-works", nil)
	if code != 301 || !strings.Contains(string(b), `"redirect_slug":"gujarat-glaze"`) || hdr.Get("Location") == "" {
		t.Fatalf("slug redirect = %d %s", code, b)
	}

	// Unpublish -> 410 Gone, absent from search and sitemap.
	admin.must("POST", "/api/v1/admin/companies/"+approved.CompanyID+"/unpublish", map[string]string{"reason": "test"}, 200, nil)
	if code, _, _ := pub.do("GET", "/api/v1/public/companies/gujarat-glaze", nil); code != 410 {
		t.Fatalf("unpublished profile = %d, want 410", code)
	}
	assertNotPublic(t, pub, "gujarat-glaze")
}

func assertNotPublic(t *testing.T, c *client, slug string) {
	t.Helper()
	if code, _, _ := c.do("GET", "/api/v1/public/companies/"+slug, nil); code == 200 {
		t.Fatalf("%s should not be public", slug)
	}
	for _, p := range []string{"/api/v1/public/companies?limit=100", "/api/v1/public/sitemap"} {
		_, b, _ := c.do("GET", p, nil)
		if strings.Contains(string(b), slug) {
			t.Fatalf("%s leaks %s: %s", p, slug, b)
		}
	}
	// Synthetic seed records are private by default.
	_, b, _ := c.do("GET", "/api/v1/public/companies?limit=100", nil)
	if strings.Contains(string(b), "Synthetic") {
		t.Fatal("synthetic private records leaked publicly")
	}
}

func TestPublishBlockedWithoutPublicPermission(t *testing.T) {
	e := newEnv(t)
	pub := e.anon()
	var created struct{ ID string }
	pub.must("POST", "/api/v1/supplier-submissions", submission("Private Pipes Co", false, false), 202, &created)
	admin := e.login("admin")
	var approved struct {
		CompanyID string `json:"company_id"`
	}
	admin.must("POST", "/api/v1/admin/submissions/"+created.ID+"/approve", nil, 200, &approved)
	b := admin.must("POST", "/api/v1/admin/companies/"+approved.CompanyID+"/publish", nil, 422, nil)
	if !strings.Contains(string(b), "publish_blocked") || !strings.Contains(string(b), "public display") {
		t.Fatalf("publish should be blocked with reasons: %s", b)
	}
	assertNotPublic(t, pub, "private-pipes-co")
}

func TestSubmissionValidationAndAntiSpam(t *testing.T) {
	e := newEnv(t)
	pub := e.anon()
	bad := submission("X Co", true, true)
	bad["consents"] = map[string]any{"privacy_notice_acknowledged": false, "public_listing": true, "commercial_api": true}
	bad["roles"] = []string{"wizard"}
	b := pub.must("POST", "/api/v1/supplier-submissions", bad, 422, nil)
	for _, f := range []string{"consents.privacy_notice_acknowledged", "roles"} {
		if !strings.Contains(string(b), f) {
			t.Errorf("expected field error %s: %s", f, b)
		}
	}
	spam := submission("Spam Co", true, true)
	spam["website_confirm"] = "http://spam.example"
	pub.must("POST", "/api/v1/supplier-submissions", spam, 202, nil)
	fast := submission("Fast Co", true, true)
	fast["form_started_at"] = time.Now().UnixMilli()
	pub.must("POST", "/api/v1/supplier-submissions", fast, 202, nil)
	var n int
	_ = e.st.Pool.QueryRow(context.Background(), `SELECT count(*) FROM supplier_submissions`).Scan(&n)
	if n != 0 {
		t.Fatalf("spam submissions were stored: %d", n)
	}
	unknown := submission("Y Co", true, true)
	unknown["extra"] = 1
	pub.must("POST", "/api/v1/supplier-submissions", unknown, 400, nil)
}

const importCSV = `Firm Name,City,State,Pin Code,Website,Category,Nature of Business,Products,Mobile,E-mail
Kalin Weaves Pvt Ltd,Bhadohi,Uttar Pradesh,221401,https://kalin-weaves-synthetic.example,Hand-knotted rugs,Manufacturer;Exporter,Wool rug,98390 11111,info@kalin.example
Brand New Bricks,Patna,Bihar,800001,brandnewbricks.example,Clay bricks,Manufacturing,Red brick,,owner.name@brandnewbricks.example
,Nowhere,XX,000000,,,,,,
`

func TestImportPipeline(t *testing.T) {
	e := newEnv(t)
	admin := e.login("admin")

	// Unknown rights: staged but cannot commit.
	var job struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		TotalRows     int    `json:"total_rows"`
		ErrorRows     int    `json:"error_rows"`
		DuplicateRows int    `json:"duplicate_rows"`
	}
	admin.must("POST", "/api/v1/admin/imports", csvUpload("legacy.csv", importCSV, map[string]string{
		"source": `{"source_type":"authorized_import","provider":"legacy spreadsheet"}`}), 201, &job)
	if job.Status != "staged" || job.TotalRows != 3 || job.ErrorRows != 1 || job.DuplicateRows != 1 {
		t.Fatalf("job = %+v", job)
	}
	b := admin.must("POST", "/api/v1/admin/imports/"+job.ID+"/commit", map[string]any{"dry_run": false, "include_clean_pending": true}, 422, nil)
	if !strings.Contains(string(b), "source_not_permitted") {
		t.Fatalf("commit from unknown-rights source should be blocked: %s", b)
	}
	// Re-uploading the same file for the same source is rejected.
	var src struct {
		Source struct{ ID string } `json:"source"`
	}
	admin.must("GET", "/api/v1/admin/imports/"+job.ID, nil, 200, &src)
	admin.must("POST", "/api/v1/admin/imports", csvUpload("legacy.csv", importCSV, map[string]string{"source_id": src.Source.ID}), 409, nil)

	// With internal-use rights.
	admin.must("POST", "/api/v1/admin/imports", csvUpload("legacy.csv", importCSV, map[string]string{
		"source": `{"source_type":"authorized_import","provider":"own research sheet","allowed_internal_use":true,"license_reference":"founder-collected 2025"}`}), 201, &job)
	var rows struct {
		Data []struct {
			ID                  string `json:"id"`
			RowNumber           int    `json:"row_number"`
			DuplicateCandidates []struct {
				CompanyID string `json:"company_id"`
			} `json:"duplicate_candidates"`
		} `json:"data"`
	}
	admin.must("GET", "/api/v1/admin/imports/"+job.ID+"/rows?filter=duplicates", nil, 200, &rows)
	if len(rows.Data) != 1 || rows.Data[0].RowNumber != 1 {
		t.Fatalf("expected Kalin Weaves to match the synthetic record: %+v", rows.Data)
	}
	dupRow := rows.Data[0]

	var dry struct {
		Created, Merged, Skipped int
	}
	admin.must("POST", "/api/v1/admin/imports/"+job.ID+"/commit", map[string]any{"include_clean_pending": true}, 200, &dry)
	if dry.Created != 1 || dry.Skipped != 2 {
		t.Fatalf("dry run = %+v", dry)
	}
	var count int
	_ = e.st.Pool.QueryRow(context.Background(), `SELECT count(*) FROM companies WHERE display_name = 'Brand New Bricks'`).Scan(&count)
	if count != 0 {
		t.Fatal("dry run wrote data")
	}

	admin.must("PATCH", "/api/v1/admin/imports/"+job.ID+"/rows/"+dupRow.ID, map[string]any{"disposition": "merged",
		"matched_company_id": dupRow.DuplicateCandidates[0].CompanyID}, 200, nil)
	var res struct{ Created, Merged, Skipped int }
	admin.must("POST", "/api/v1/admin/imports/"+job.ID+"/commit", map[string]any{"dry_run": false, "include_clean_pending": true}, 200, &res)
	if res.Created != 1 || res.Merged != 1 || res.Skipped != 1 {
		t.Fatalf("commit = %+v", res)
	}
	var personalPlain, personalEnc int
	_ = e.st.Pool.QueryRow(context.Background(), `SELECT count(*) FILTER (WHERE value LIKE '%owner.name%'),
		count(*) FILTER (WHERE contact_type = 'individual' AND value_encrypted IS NOT NULL) FROM contacts`).Scan(&personalPlain, &personalEnc)
	if personalPlain != 0 || personalEnc < 2 {
		t.Fatalf("personal contacts must be encrypted: plain=%d enc=%d", personalPlain, personalEnc)
	}
	report := admin.must("GET", "/api/v1/admin/imports/"+job.ID+"/report.csv", nil, 200, nil)
	if !strings.Contains(string(report), "display_name: is required") {
		t.Fatalf("report missing errors: %s", report)
	}
	admin.must("POST", "/api/v1/admin/imports/"+job.ID+"/rollback", nil, 200, nil)
	_ = e.st.Pool.QueryRow(context.Background(), `SELECT count(*) FROM companies WHERE display_name = 'Brand New Bricks' AND deleted_at IS NULL`).Scan(&count)
	if count != 0 {
		t.Fatal("rollback did not remove created company")
	}
	_ = e.st.Pool.QueryRow(context.Background(), `SELECT count(*) FROM products WHERE name = 'Wool rug' AND deleted_at IS NULL`).Scan(&count)
	if count != 0 {
		t.Fatal("rollback did not remove merged child records")
	}
}

func TestExportsArePermissionFiltered(t *testing.T) {
	e := newEnv(t)
	pub := e.anon()
	var a, b struct{ ID string }
	pub.must("POST", "/api/v1/supplier-submissions", submission("Commercial Ceramics", true, true), 202, &a)
	pub.must("POST", "/api/v1/supplier-submissions", submission("Public Only Ceramics", true, false), 202, &b)
	owner := e.login("owner")
	owner.must("POST", "/api/v1/admin/submissions/"+a.ID+"/approve", nil, 200, nil)
	owner.must("POST", "/api/v1/admin/submissions/"+b.ID+"/approve", nil, 200, nil)

	csv := string(owner.must("POST", "/api/v1/admin/exports", map[string]string{"scope": "commercial"}, 200, nil))
	if !strings.Contains(csv, "Commercial Ceramics") || strings.Contains(csv, "Public Only Ceramics") || strings.Contains(csv, "Synthetic") {
		t.Fatalf("commercial export not filtered:\n%s", csv)
	}
	internal := string(owner.must("POST", "/api/v1/admin/exports", map[string]string{"scope": "internal"}, 200, nil))
	if !strings.Contains(internal, "(Synthetic)") {
		t.Fatal("internal export should include internally permitted records")
	}
	for _, out := range []string{csv, internal} {
		if strings.Contains(out, "priya") {
			t.Fatal("export leaks personal contact")
		}
	}
	public := string(owner.must("POST", "/api/v1/admin/exports", map[string]string{"scope": "public"}, 200, nil))
	if strings.Count(public, "\n") != 1 {
		t.Fatalf("nothing is published, public export should be header only:\n%s", public)
	}
	var logs struct {
		Data []struct{ Action string } `json:"data"`
	}
	owner.must("GET", "/api/v1/admin/audit-logs?action=export", nil, 200, &logs)
	if len(logs.Data) != 3 {
		t.Fatalf("exports must be audit logged, got %d", len(logs.Data))
	}
}

func TestDuplicateMergeAndPrivacyWorkflow(t *testing.T) {
	e := newEnv(t)
	pub := e.anon()
	var sub struct{ ID string }
	s := submission("Morvi Vitrified", true, false)
	pub.must("POST", "/api/v1/supplier-submissions", s, 202, &sub)
	owner := e.login("owner")
	var approved struct {
		CompanyID string `json:"company_id"`
	}
	owner.must("POST", "/api/v1/admin/submissions/"+sub.ID+"/approve", nil, 200, &approved)

	var dups struct {
		Data []struct {
			ID       string              `json:"id"`
			CompanyA struct{ ID string } `json:"company_a"`
			CompanyB struct{ ID string } `json:"company_b"`
		} `json:"data"`
	}
	owner.must("GET", "/api/v1/admin/duplicates", nil, 200, &dups)
	if len(dups.Data) == 0 {
		t.Fatal("approval should have flagged the synthetic Morvi Vitrified as a possible duplicate")
	}
	pair := dups.Data[0]
	keep := "a"
	if pair.CompanyB.ID == approved.CompanyID {
		keep = "b"
	}
	owner.must("POST", "/api/v1/admin/duplicates/"+pair.ID+"/resolve", map[string]string{"action": "merge", "keep": keep}, 200, nil)
	var merged struct {
		Facilities []any `json:"facilities"`
		Provenance []struct {
			Source struct{ Provider string } `json:"source"`
		} `json:"provenance"`
	}
	owner.must("GET", "/api/v1/admin/companies/"+approved.CompanyID, nil, 200, &merged)
	providers := map[string]bool{}
	for _, p := range merged.Provenance {
		providers[p.Source.Provider] = true
	}
	if !providers["synthetic-seed"] || !providers["supplier-registration-form"] || len(merged.Facilities) < 1 {
		t.Fatalf("merge must preserve provenance from both records: %v", providers)
	}
	owner.must("POST", "/api/v1/admin/companies/"+approved.CompanyID+"/publish", nil, 200, nil)
	pub.must("GET", "/api/v1/public/companies/morvi-vitrified", nil, 200, nil)
	// The merged-away synthetic record's slug redirects to the survivor.
	if code, _, _ := pub.do("GET", "/api/v1/public/companies/morvi-vitrified-synthetic", nil); code != 301 {
		t.Fatalf("merged slug should redirect, got %d", code)
	}

	// Supplier withdraws public listing.
	var pr struct{ ID string }
	pub.must("POST", "/api/v1/privacy-requests", map[string]string{"request_type": "withdraw_public_listing",
		"requester_name": "Priya", "email": "priya.private@person.example", "company_reference": "https://site.test/suppliers/morvi-vitrified"}, 202, &pr)
	var rec struct {
		CompanyID *string `json:"company_id"`
		Status    string  `json:"status"`
	}
	owner.must("POST", "/api/v1/admin/privacy-requests/"+pr.ID+"/resolve", map[string]string{"status": "resolved", "action": "withdraw_public"}, 200, &rec)
	if rec.CompanyID == nil || *rec.CompanyID != approved.CompanyID || rec.Status != "resolved" {
		t.Fatalf("privacy request not linked/resolved: %+v", rec)
	}
	if code, _, _ := pub.do("GET", "/api/v1/public/companies/morvi-vitrified", nil); code != 410 {
		t.Fatalf("withdrawn listing should be gone, got %d", code)
	}
	b := owner.must("POST", "/api/v1/admin/companies/"+approved.CompanyID+"/publish", nil, 422, nil)
	if !strings.Contains(string(b), "withdrawn") {
		t.Fatalf("re-publishing after withdrawal must be blocked: %s", b)
	}

	// Erasure.
	pub.must("POST", "/api/v1/privacy-requests", map[string]string{"request_type": "removal", "requester_name": "Priya",
		"email": "priya.private@person.example", "company_reference": "morvi-vitrified"}, 202, &pr)
	owner.must("POST", "/api/v1/admin/privacy-requests/"+pr.ID+"/resolve", map[string]string{"status": "resolved", "action": "erase_company"}, 200, nil)
	owner.must("GET", "/api/v1/admin/companies/"+approved.CompanyID, nil, 404, nil)
	var leftover int
	_ = e.st.Pool.QueryRow(context.Background(), `SELECT count(*) FROM supplier_submissions WHERE payload::text LIKE '%Morvi%'`).Scan(&leftover)
	if leftover != 0 {
		t.Fatal("erasure must anonymise the submission payload")
	}
}

func TestRevokingSourceUnpublishes(t *testing.T) {
	e := newEnv(t)
	pub := e.anon()
	var sub struct{ ID string }
	pub.must("POST", "/api/v1/supplier-submissions", submission("Revocable Tiles", true, false), 202, &sub)
	owner := e.login("owner")
	var approved struct {
		CompanyID string `json:"company_id"`
	}
	owner.must("POST", "/api/v1/admin/submissions/"+sub.ID+"/approve", nil, 200, &approved)
	owner.must("POST", "/api/v1/admin/companies/"+approved.CompanyID+"/publish", nil, 200, nil)
	var sourceID string
	_ = e.st.Pool.QueryRow(context.Background(), `SELECT source_id FROM supplier_submissions WHERE id = $1`, sub.ID).Scan(&sourceID)

	admin := e.login("admin")
	admin.must("PATCH", "/api/v1/admin/sources/"+sourceID, map[string]any{"allowed_public_display": false}, 403, nil)
	_, _, hdr := owner.do("PATCH", "/api/v1/admin/sources/"+sourceID, map[string]any{"allowed_public_display": false})
	if hdr.Get("X-Unpublished-Count") != "1" {
		t.Fatalf("expected 1 company unpublished, header=%q", hdr.Get("X-Unpublished-Count"))
	}
	if code, _, _ := pub.do("GET", "/api/v1/public/companies/revocable-tiles", nil); code == 200 {
		t.Fatal("company still public after source permission revoked")
	}
}

func TestVerificationIndependentOfPublication(t *testing.T) {
	e := newEnv(t)
	reviewer := e.login("reviewer")
	var list struct {
		Data []struct{ ID string } `json:"data"`
	}
	reviewer.must("GET", "/api/v1/admin/companies?q=kalin", nil, 200, &list)
	if len(list.Data) == 0 {
		t.Fatal("search found nothing")
	}
	var c struct {
		VerificationStatus string     `json:"verification_status"`
		PublicationStatus  string     `json:"publication_status"`
		NextReviewAt       *time.Time `json:"next_review_at"`
	}
	reviewer.must("POST", "/api/v1/admin/companies/"+list.Data[0].ID+"/verify", map[string]any{"method": "supplier_confirmation", "result": "confirmed", "next_review_days": 30}, 200, &c)
	if c.VerificationStatus != "verified" || c.PublicationStatus != "approved_private" || c.NextReviewAt == nil {
		t.Fatalf("verify = %+v", c)
	}
	if _, err := e.st.Pool.Exec(context.Background(), `UPDATE companies SET next_review_at = now() - interval '1 day' WHERE id = $1`, list.Data[0].ID); err != nil {
		t.Fatal(err)
	}
	rep, err := e.st.RunMaintenance(context.Background())
	if err != nil || rep.MarkedStale != 1 {
		t.Fatalf("maintenance = %+v %v", rep, err)
	}
}

func TestPublicCategoryAndStateEndpoints(t *testing.T) {
	e := newEnv(t)
	c := e.anon()
	var tree struct {
		Data []struct {
			Slug     string `json:"slug"`
			Children []any  `json:"children"`
		} `json:"data"`
	}
	c.must("GET", "/api/v1/categories/tree", nil, 200, &tree)
	if len(tree.Data) != 14 {
		t.Fatalf("top-level categories = %d", len(tree.Data))
	}
	var detail struct {
		Ancestors []struct{ Slug string } `json:"ancestors"`
	}
	c.must("GET", "/api/v1/categories/vitrified-tiles", nil, 200, &detail)
	if len(detail.Ancestors) != 2 || detail.Ancestors[0].Slug != "building-and-construction-materials" {
		t.Fatalf("ancestors = %+v", detail.Ancestors)
	}
	var states struct {
		Data []struct {
			PublishedCompanyCount int `json:"published_company_count"`
		} `json:"data"`
	}
	c.must("GET", "/api/v1/states", nil, 200, &states)
	for _, s := range states.Data {
		if s.PublishedCompanyCount != 0 {
			t.Fatal("private synthetic data must not count towards public state totals")
		}
	}
}
