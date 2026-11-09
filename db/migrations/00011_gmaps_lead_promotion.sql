-- +goose Up

-- 'public_web_listing': data collected from public web listings (e.g. Google Maps via the scraper).
-- Internal use only by default: the listing site's terms usually do not grant reuse or redistribution.
ALTER TABLE data_sources DROP CONSTRAINT data_sources_source_type_check;
ALTER TABLE data_sources ADD CONSTRAINT data_sources_source_type_check CHECK (source_type IN (
  'self_submission', 'authorized_import', 'official_site',
  'licensed_feed', 'permitted_api_lookup', 'manual_verification', 'public_web_listing'));

-- A promoted lead points at the company created from it.
ALTER TABLE gmaps_leads ADD COLUMN promoted_company_id uuid REFERENCES companies(id) ON DELETE SET NULL;
CREATE INDEX gmaps_leads_state_idx ON gmaps_leads (state_code, status);

-- +goose Down
DROP INDEX IF EXISTS gmaps_leads_state_idx;
ALTER TABLE gmaps_leads DROP COLUMN IF EXISTS promoted_company_id;
UPDATE data_sources SET source_type = 'manual_verification' WHERE source_type = 'public_web_listing';
ALTER TABLE data_sources DROP CONSTRAINT data_sources_source_type_check;
ALTER TABLE data_sources ADD CONSTRAINT data_sources_source_type_check CHECK (source_type IN (
  'self_submission', 'authorized_import', 'official_site',
  'licensed_feed', 'permitted_api_lookup', 'manual_verification'));
