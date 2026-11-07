-- +goose Up

CREATE TABLE companies (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  legal_name          text CHECK (legal_name IS NULL OR length(btrim(legal_name)) BETWEEN 2 AND 300),
  display_name        text NOT NULL CHECK (length(btrim(display_name)) BETWEEN 2 AND 300),
  slug                text NOT NULL CHECK (is_valid_slug(slug)),
  normalized_name     text NOT NULL CHECK (length(normalized_name) > 0),
  description         text CHECK (description IS NULL OR length(description) <= 5000),
  website_url         text CHECK (website_url IS NULL OR website_url ~* '^https?://[^\s/$.?#].[^\s]*$'),
  website_domain      text CHECK (website_domain IS NULL OR website_domain = lower(website_domain)),
  establishment_year  smallint CHECK (establishment_year IS NULL OR establishment_year BETWEEN 1800 AND 2100),
  business_status     text NOT NULL DEFAULT 'unknown'
                      CHECK (business_status IN ('active', 'inactive', 'closed', 'unknown')),
  -- Workflow: draft -> submitted -> under_review -> approved_private -> published,
  -- plus rejected, needs_changes, archived. Publishing does NOT imply verification.
  publication_status  text NOT NULL DEFAULT 'draft'
                      CHECK (publication_status IN ('draft', 'submitted', 'under_review', 'approved_private',
                                                    'published', 'rejected', 'needs_changes', 'archived')),
  verification_status text NOT NULL DEFAULT 'unverified'
                      CHECK (verification_status IN ('unverified', 'partially_verified', 'verified', 'stale')),
  primary_category_id uuid REFERENCES categories(id) ON DELETE SET NULL,
  published_at        timestamptz,
  last_verified_at    timestamptz,
  next_review_at      timestamptz,
  merged_into_id      uuid REFERENCES companies(id),
  -- Company-level withdrawal of optional permissions (supplier request). These override any
  -- source-level permission for this company without affecting other companies from the same source.
  public_listing_withdrawn_at timestamptz,
  commercial_api_withdrawn_at timestamptz,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  deleted_at          timestamptz,
  CONSTRAINT companies_published_has_timestamp CHECK (publication_status <> 'published' OR published_at IS NOT NULL),
  CONSTRAINT companies_deleted_not_published CHECK (deleted_at IS NULL OR publication_status <> 'published'),
  CONSTRAINT companies_verified_has_timestamp CHECK (verification_status <> 'verified' OR last_verified_at IS NOT NULL),
  CONSTRAINT companies_not_merged_into_self CHECK (merged_into_id IS NULL OR merged_into_id <> id),
  CONSTRAINT companies_withdrawn_not_published CHECK (public_listing_withdrawn_at IS NULL OR publication_status <> 'published')
);
-- Slugs stay reserved after soft delete so old URLs can 410/redirect instead of being reused.
CREATE UNIQUE INDEX companies_slug_key ON companies (slug);
CREATE INDEX companies_normalized_name_trgm ON companies USING gin (normalized_name gin_trgm_ops) WHERE deleted_at IS NULL;
CREATE INDEX companies_website_domain_idx ON companies (website_domain) WHERE website_domain IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX companies_publication_idx ON companies (publication_status, updated_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX companies_next_review_idx ON companies (next_review_at) WHERE next_review_at IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX companies_primary_category_idx ON companies (primary_category_id);
CREATE INDEX companies_published_name_idx ON companies (display_name)
  WHERE publication_status = 'published' AND deleted_at IS NULL;
CREATE TRIGGER companies_updated_at BEFORE UPDATE ON companies
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE slug_redirects (
  old_slug   text PRIMARY KEY CHECK (is_valid_slug(old_slug)),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX slug_redirects_company_idx ON slug_redirects (company_id);

CREATE TABLE company_roles (
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  role_code  text NOT NULL REFERENCES business_roles(code) ON UPDATE CASCADE,
  source_id  uuid REFERENCES data_sources(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (company_id, role_code)
);
CREATE INDEX company_roles_role_idx ON company_roles (role_code);

CREATE TABLE facilities (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id          uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  facility_type       text NOT NULL DEFAULT 'factory'
                      CHECK (facility_type IN ('factory', 'office', 'registered_office', 'warehouse',
                                               'showroom', 'workshop', 'laboratory', 'other')),
  name                text,
  address_line1       text,
  address_line2       text,
  locality            text,
  city                text,
  district            text,
  state_code          char(2) REFERENCES indian_states(code),
  postal_code         text,
  country_code        char(2) NOT NULL DEFAULT 'IN' CHECK (country_code ~ '^[A-Z]{2}$'),
  latitude            numeric(9, 6) CHECK (latitude IS NULL OR latitude BETWEEN -90 AND 90),
  longitude           numeric(9, 6) CHECK (longitude IS NULL OR longitude BETWEEN -180 AND 180),
  -- Coordinates must carry their own source; Google-derived coordinates must not be stored
  -- beyond what the provider permits.
  geo_source_id       uuid REFERENCES data_sources(id),
  is_primary          boolean NOT NULL DEFAULT false,
  source_id           uuid REFERENCES data_sources(id),
  verification_status text NOT NULL DEFAULT 'unverified'
                      CHECK (verification_status IN ('unverified', 'partially_verified', 'verified', 'stale')),
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  deleted_at          timestamptz,
  CONSTRAINT facilities_geo_pair CHECK ((latitude IS NULL) = (longitude IS NULL)),
  CONSTRAINT facilities_geo_has_source CHECK (latitude IS NULL OR geo_source_id IS NOT NULL),
  CONSTRAINT facilities_india_pin CHECK (postal_code IS NULL OR country_code <> 'IN' OR postal_code ~ '^[1-9][0-9]{5}$'),
  CONSTRAINT facilities_india_state CHECK (state_code IS NULL OR country_code = 'IN')
);
CREATE UNIQUE INDEX facilities_one_primary ON facilities (company_id) WHERE is_primary AND deleted_at IS NULL;
CREATE INDEX facilities_company_idx ON facilities (company_id) WHERE deleted_at IS NULL;
CREATE INDEX facilities_state_city_idx ON facilities (state_code, lower(city)) WHERE deleted_at IS NULL;
CREATE TRIGGER facilities_updated_at BEFORE UPDATE ON facilities
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE company_categories (
  company_id  uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  category_id uuid NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
  source_id   uuid REFERENCES data_sources(id),
  confidence  numeric(3, 2) CHECK (confidence IS NULL OR confidence BETWEEN 0 AND 1),
  verified_at timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (company_id, category_id)
);
CREATE INDEX company_categories_category_idx ON company_categories (category_id, company_id);

CREATE TABLE products (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id      uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  category_id     uuid REFERENCES categories(id) ON DELETE SET NULL,
  name            text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 300),
  normalized_name text NOT NULL,
  description     text CHECK (description IS NULL OR length(description) <= 3000),
  unit            text,
  attributes      jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(attributes) = 'object'),
  source_id       uuid REFERENCES data_sources(id),
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz
);
CREATE INDEX products_company_idx ON products (company_id) WHERE deleted_at IS NULL;
CREATE INDEX products_category_idx ON products (category_id) WHERE deleted_at IS NULL;
CREATE INDEX products_normalized_name_trgm ON products USING gin (normalized_name gin_trgm_ops) WHERE deleted_at IS NULL;
CREATE TRIGGER products_updated_at BEFORE UPDATE ON products
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE company_capabilities (
  company_id          uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  capability_id       uuid NOT NULL REFERENCES capabilities(id) ON DELETE RESTRICT,
  source_id           uuid REFERENCES data_sources(id),
  attributes          jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(attributes) = 'object'),
  -- "unverified" means "claimed but not checked" - never the same as "does not have".
  verification_status text NOT NULL DEFAULT 'unverified'
                      CHECK (verification_status IN ('unverified', 'partially_verified', 'verified', 'stale')),
  created_at          timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (company_id, capability_id)
);
CREATE INDEX company_capabilities_capability_idx ON company_capabilities (capability_id);

-- Private documents (verification evidence). Never publicly exposed in v1.
CREATE TABLE documents (
  id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id           uuid REFERENCES companies(id) ON DELETE CASCADE,
  submission_id        uuid,
  private_object_key   text NOT NULL UNIQUE,
  original_filename    text,
  mime_type            text NOT NULL CHECK (mime_type IN ('application/pdf', 'image/jpeg', 'image/png', 'image/webp')),
  size_bytes           bigint NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 10485760),
  checksum_sha256      text NOT NULL CHECK (checksum_sha256 ~ '^[0-9a-f]{64}$'),
  access_level         text NOT NULL DEFAULT 'private' CHECK (access_level IN ('private', 'restricted')),
  retention_policy     text NOT NULL DEFAULT 'delete_after_verification_plus_1y',
  retention_expires_at timestamptz,
  uploaded_by          uuid REFERENCES admin_users(id),
  uploaded_at          timestamptz NOT NULL DEFAULT now(),
  deleted_at           timestamptz
);
CREATE INDEX documents_company_idx ON documents (company_id) WHERE deleted_at IS NULL;

CREATE TABLE certifications (
  id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id           uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  certification_type   text NOT NULL CHECK (length(btrim(certification_type)) > 0),
  issuer               text,
  certificate_number   text,
  issued_at            date,
  expires_at           date,
  -- Never defaults to verified. "claimed"/"unverified" are distinct from "rejected".
  status               text NOT NULL DEFAULT 'unverified'
                       CHECK (status IN ('claimed', 'unverified', 'verified', 'expired', 'revoked', 'rejected')),
  evidence_document_id uuid REFERENCES documents(id) ON DELETE SET NULL,
  source_id            uuid REFERENCES data_sources(id),
  verified_at          timestamptz,
  verified_by          uuid REFERENCES admin_users(id),
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT certifications_dates CHECK (expires_at IS NULL OR issued_at IS NULL OR expires_at >= issued_at),
  CONSTRAINT certifications_verified_has_reviewer CHECK (status <> 'verified' OR (verified_at IS NOT NULL AND verified_by IS NOT NULL))
);
CREATE INDEX certifications_company_idx ON certifications (company_id);
CREATE TRIGGER certifications_updated_at BEFORE UPDATE ON certifications
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Contacts. Individual (personal) contacts are stored encrypted only and can never be public.
CREATE TABLE contacts (
  id                             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id                     uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  contact_type                   text NOT NULL CHECK (contact_type IN (
                                   'public_business_email', 'switchboard', 'sales_department', 'individual')),
  channel                        text NOT NULL CHECK (channel IN ('email', 'phone', 'url')),
  label                          text,
  value                          text,
  value_encrypted                bytea,
  value_hash                     text NOT NULL,
  is_public                      boolean NOT NULL DEFAULT false,
  lawful_basis_or_permission_ref text,
  source_id                      uuid REFERENCES data_sources(id),
  verified_at                    timestamptz,
  created_at                     timestamptz NOT NULL DEFAULT now(),
  updated_at                     timestamptz NOT NULL DEFAULT now(),
  deleted_at                     timestamptz,
  CONSTRAINT contacts_exactly_one_value CHECK ((value IS NULL) <> (value_encrypted IS NULL)),
  CONSTRAINT contacts_individual_protected CHECK (
    contact_type <> 'individual' OR (value IS NULL AND value_encrypted IS NOT NULL AND NOT is_public)),
  CONSTRAINT contacts_public_requires_basis CHECK (NOT is_public OR lawful_basis_or_permission_ref IS NOT NULL)
);
CREATE INDEX contacts_company_idx ON contacts (company_id) WHERE deleted_at IS NULL;
CREATE INDEX contacts_value_hash_idx ON contacts (value_hash) WHERE deleted_at IS NULL;
CREATE TRIGGER contacts_updated_at BEFORE UPDATE ON contacts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Field-level provenance: which source supplied which field, and whether that
-- specific use is permitted.
CREATE TABLE field_provenance (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id        uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  entity_type       text NOT NULL CHECK (entity_type IN (
                      'company', 'facility', 'product', 'contact', 'certification', 'capability', 'category', 'role')),
  entity_id         uuid NOT NULL,
  field_name        text NOT NULL CHECK (field_name ~ '^[a-z][a-z0-9_]*$'),
  source_id         uuid NOT NULL REFERENCES data_sources(id),
  observed_at       timestamptz NOT NULL DEFAULT now(),
  permission_status text NOT NULL DEFAULT 'unknown'
                    CHECK (permission_status IN ('unknown', 'permitted', 'restricted', 'withdrawn')),
  confidence        numeric(3, 2) CHECK (confidence IS NULL OR confidence BETWEEN 0 AND 1),
  created_at        timestamptz NOT NULL DEFAULT now(),
  UNIQUE (entity_type, entity_id, field_name, source_id)
);
CREATE INDEX field_provenance_company_idx ON field_provenance (company_id);
CREATE INDEX field_provenance_source_idx ON field_provenance (source_id);
-- Lookups by entity (e.g. "is this facility's city publicly permitted").
CREATE INDEX field_provenance_entity_idx ON field_provenance (entity_id, entity_type, field_name) WHERE permission_status = 'permitted';

CREATE TABLE verification_events (
  id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id           uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  entity_type          text NOT NULL DEFAULT 'company',
  entity_id            uuid,
  field_name           text,
  method               text NOT NULL CHECK (method IN (
                         'supplier_confirmation', 'official_document', 'website_check', 'manual_review', 'phone_call')),
  result               text NOT NULL CHECK (result IN ('confirmed', 'partially_confirmed', 'not_confirmed', 'inconclusive')),
  evidence_document_id uuid REFERENCES documents(id) ON DELETE SET NULL,
  notes                text,
  checked_at           timestamptz NOT NULL DEFAULT now(),
  checked_by           uuid NOT NULL REFERENCES admin_users(id),
  next_review_at       timestamptz,
  created_at           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX verification_events_company_idx ON verification_events (company_id, checked_at DESC);
CREATE INDEX verification_events_next_review_idx ON verification_events (next_review_at) WHERE next_review_at IS NOT NULL;

-- Defence in depth: the database itself refuses to publish a company unless its
-- display name is backed by a source that permits public display.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION companies_enforce_publication() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.publication_status = 'published' AND (TG_OP = 'INSERT' OR OLD.publication_status IS DISTINCT FROM 'published') THEN
    IF NOT EXISTS (
      SELECT 1
      FROM field_provenance fp
      JOIN data_sources ds ON ds.id = fp.source_id
      WHERE fp.company_id = NEW.id
        AND fp.entity_type = 'company'
        AND fp.entity_id = NEW.id
        AND fp.field_name = 'display_name'
        AND fp.permission_status = 'permitted'
        AND ds.allowed_public_display
        AND (ds.retention_expires_at IS NULL OR ds.retention_expires_at > now())
    ) THEN
      RAISE EXCEPTION 'company % cannot be published: display_name lacks public display permission', NEW.id
        USING ERRCODE = 'check_violation', CONSTRAINT = 'companies_publication_permission';
    END IF;
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER companies_publication_guard BEFORE INSERT OR UPDATE OF publication_status ON companies
  FOR EACH ROW EXECUTE FUNCTION companies_enforce_publication();

-- +goose Down
DROP TRIGGER IF EXISTS companies_publication_guard ON companies;
DROP FUNCTION IF EXISTS companies_enforce_publication();
DROP TABLE IF EXISTS verification_events;
DROP TABLE IF EXISTS field_provenance;
DROP TABLE IF EXISTS contacts;
DROP TABLE IF EXISTS certifications;
DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS company_capabilities;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS company_categories;
DROP TABLE IF EXISTS facilities;
DROP TABLE IF EXISTS company_roles;
DROP TABLE IF EXISTS slug_redirects;
DROP TABLE IF EXISTS companies;
