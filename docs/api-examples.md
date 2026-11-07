# API examples

Full contract: `packages/openapi/openapi.yaml` (served at `/api/v1/openapi.yaml` outside production).
Base URL below: the web origin `http://localhost:3000`, which proxies `/api/*` to the Go API. You can also call the API directly on `http://localhost:${API_PORT:-8080}`.

Errors always look like:
```json
{"error":{"code":"validation_failed","message":"some fields are invalid","request_id":"…","details":{"fields":{"company.display_name":"is required"}}}}
```

## Public

```bash
curl -s localhost:3000/api/v1/categories/tree | jq '.data[].slug'
curl -s 'localhost:3000/api/v1/public/companies?category=tiles&state=gujarat&page=1&limit=20' | jq
curl -s localhost:3000/api/v1/public/companies/morvi-vitrified-tiles | jq   # 200, 301 {redirect_slug}, 404 or 410
curl -s localhost:3000/api/v1/public/sitemap | jq
```

Register a supplier (wait ≥3 s after `form_started_at`, or the anti-spam check silently drops it):
```bash
curl -s -X POST localhost:3000/api/v1/supplier-submissions \
  -H 'Content-Type: application/json' -H "Idempotency-Key: $(uuidgen)" -d @- <<JSON | jq
{
  "company": {"display_name": "Example Ceramics", "website_url": "https://example.com"},
  "roles": ["manufacturer"],
  "categories": ["vitrified-tiles"],
  "products": [{"name": "Glazed vitrified tile 600x1200"}],
  "facilities": [{"facility_type": "factory", "city": "Morbi", "state_code": "GJ", "postal_code": "363641"}],
  "business_contacts": [{"contact_type": "sales_department", "channel": "email", "value": "sales@example.com"}],
  "submitter": {"name": "Your Name", "email": "you@example.com"},
  "consents": {"privacy_notice_acknowledged": true, "public_listing": true, "commercial_api": false},
  "privacy_notice_version": "2026-11-v1",
  "form_started_at": $(( ($(date +%s) - 60) * 1000 ))
}
JSON
```

## Admin (cookie session + CSRF)

```bash
JAR=$(mktemp)
CSRF=$(curl -s -c $JAR -X POST localhost:3000/api/v1/admin/login -H 'Content-Type: application/json' \
  -H 'Origin: http://localhost:3000' -d '{"email":"you@example.com","password":"…"}' | jq -r .csrf_token)
H=(-b $JAR -H "X-CSRF-Token: $CSRF" -H 'Origin: http://localhost:3000' -H 'Content-Type: application/json')

curl -s -b $JAR localhost:3000/api/v1/admin/stats | jq
curl -s -b $JAR 'localhost:3000/api/v1/admin/submissions?status=submitted' | jq '.data[] | {id, company_name}'
curl -s "${H[@]}" -X POST localhost:3000/api/v1/admin/submissions/<id>/approve -d '{"notes":"checked website"}' | jq
curl -s "${H[@]}" -X POST localhost:3000/api/v1/admin/companies/<company_id>/publish | jq '.publication_status, .publish_check'
curl -s "${H[@]}" -X POST localhost:3000/api/v1/admin/companies/<company_id>/verify \
  -d '{"method":"supplier_confirmation","result":"confirmed","next_review_days":365}' | jq .verification_status
```

CSV import (declare the source inline):
```bash
curl -s -b $JAR -H "X-CSRF-Token: $CSRF" -H 'Origin: http://localhost:3000' -X POST localhost:3000/api/v1/admin/imports \
  -F file=@suppliers.csv \
  -F 'source={"source_type":"authorized_import","provider":"Founder research sheet 2025","license_reference":"self-collected","allowed_internal_use":true}' | jq
curl -s -b $JAR 'localhost:3000/api/v1/admin/imports/<job>/rows?filter=duplicates' | jq
curl -s "${H[@]}" -X POST localhost:3000/api/v1/admin/imports/<job>/commit -d '{"dry_run":true,"include_clean_pending":true}' | jq
curl -s "${H[@]}" -X POST localhost:3000/api/v1/admin/imports/<job>/commit -d '{"dry_run":false,"include_clean_pending":true}' | jq
curl -s -b $JAR localhost:3000/api/v1/admin/imports/<job>/report.csv -o report.csv
```

Permission-filtered export:
```bash
curl -s "${H[@]}" -X POST localhost:3000/api/v1/admin/exports -d '{"scope":"public"}' -o public.csv
```
