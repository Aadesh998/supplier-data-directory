# Data source policy

This is the policy the software enforces. It is not legal advice; have it reviewed before launch.

## Principles

1. **Every value has a source.** Data enters only through code paths that write a `data_sources` row and `field_provenance` rows.
2. **Rights are evidenced, not assumed.** Every permission defaults to `false`. Publicly visible information is not automatically licensed for copying, display or resale.
3. **Three separate rights:** internal use, public display, commercial/API redistribution. Public or commercial rights require internal-use rights.
4. **Personal data is minimised.** Individual contacts are encrypted, never public and never exported. Business contact points are public only with a recorded basis.
5. **Restrictions survive.** Merges and imports keep each record's original `source_id`. Withdrawals override sources per company.

## Permitted collection

| Source type | Example | Default rights | Notes |
|---|---|---|---|
| `self_submission` | Supplier registration form | internal yes; public/commercial = the supplier's separate opt-ins | Strongest basis. The approval transaction sets the flags. |
| `authorized_import` | Your existing spreadsheet, a partner's list | none until you declare them | Declare provider, licence or permission reference, and the rights you can evidence. Unknown rights stay staged and cannot be committed. |
| `licensed_feed` | Purchased or licensed dataset | per contract | Record the contract reference in `license_reference`. Set `retention_expires_at` to the licence end. |
| `official_site` | The company's own website, manually reviewed | internal; public only for factual business info you are confident may be displayed | No automated crawling in v1. |
| `manual_verification` | Phone call or document check by the team | internal | Also logged as `verification_events`. |
| `permitted_api_lookup` | Google Places (disabled by default) | internal only. **The database refuses public or commercial flags.** | See below. |

Government open data (e.g. MCA company master data, Udyam/MSME lists on data.gov.in) can be imported as `authorized_import` or `licensed_feed` **after** you have read each dataset's licence (most data.gov.in datasets are under GODL-India) and recorded the attribution it requires.

## Prohibited collection

- Scraping or browser automation against Google Maps or other sites whose terms forbid it. That includes CAPTCHA bypass, proxy rotation and spreading requests to evade rate limits.
- Bulk storage of Google Maps/Places content (names, addresses, phone numbers, ratings, reviews, photos).
- Promoting any restricted or discovery-sourced data into public or commercial records.
- Collecting personal phone numbers or emails without a legal basis and a clear notice.
- Unsolicited bulk marketing (email, SMS, WhatsApp blasts).

## Google Places boundary

Checked on 7 Nov 2026 against Google's Places API policies page
(https://developers.google.com/maps/documentation/places/web-service/policies):

- Place IDs: *"You can therefore store place ID values indefinitely."* → `discovery_leads` stores only `provider_place_id` plus our own notes.
- Other content: *"Cache or store Google Maps Content only as expressly permitted under the Google Maps Platform Service Specific Terms."* Lat/lng may be cached *"for up to 30 consecutive calendar days"*. Also *"Don't create content based on Google Maps Content"* and *"Don't export, extract, or scrape Google Maps Content for use outside the Services."* → we store no names, addresses or coordinates from Places.
- Attribution: when shown without a Google map, display the Google Maps logo or the unmodified text "Google Maps". The admin discovery page shows "Google Maps" attribution with results.

**Uncertainty:** the Service Specific Terms page (https://cloud.google.com/maps-platform/terms/maps-service-terms) could not be read in full when this was written. Read it yourself before enabling the adapter, and check current pricing (the adapter requests the Pro field set: id, displayName, formattedAddress, googleMapsUri).

How the adapter is constrained (`internal/discovery/google.go`):
- off unless `GOOGLE_PLACES_ENABLED=true` and a key is set
- field mask limited to four fields
- API key sent in a header, never the URL
- local daily cap (`GOOGLE_PLACES_DAILY_CAP`) on top of Google Cloud quotas
- results returned with `Cache-Control: no-store` and never written to the database
- every search audit-logged
- no live calls in tests (an `httptest` mock is used)

Intended workflow: search → recognise a business → save its place ID as a lead → contact the business personally → it registers itself (`self_submission`). The directory record then comes from the business, not from Google.

## Discovery job list

`scripts/jobs/jobs.csv` lists 14,419 sector × location searches across all 36 states/UTs. Use it only with sources whose terms permit the use. Examples: OpenStreetMap via Overpass (ODbL: attribution and share-alike apply; respect the Overpass usage policy) or Places ID-only lookups saved as leads. Record the source and licence for anything ingested.

## Commercial API eligibility

A field may be returned to an external API customer (M8, not built) only if:
1. its provenance is `permitted` from a source with `allowed_commercial_api = true` that is unexpired, and
2. the company has not withdrawn commercial permission, and
3. it is not an individual contact.

`POST /api/v1/admin/exports {"scope":"commercial"}` already applies these rules (owner-only, audit-logged) and is the reference implementation.

## Retention

See the retention table in [data-dictionary.md](data-dictionary.md#retention-enforced-by-api-maintenance-daily-inside-serve).
