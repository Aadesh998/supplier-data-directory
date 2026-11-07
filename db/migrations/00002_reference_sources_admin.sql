-- +goose Up

-- Indian states and union territories (ISO 3166-2:IN, codes as revised in 2023).
CREATE TABLE indian_states (
  code        char(2) PRIMARY KEY CHECK (code ~ '^[A-Z]{2}$'),
  name        text NOT NULL UNIQUE,
  slug        text NOT NULL UNIQUE CHECK (is_valid_slug(slug)),
  region_type text NOT NULL CHECK (region_type IN ('state', 'union_territory'))
);

-- Admin users. Admins are deactivated, never hard-deleted (audit logs reference them).
CREATE TABLE admin_users (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email              citext NOT NULL UNIQUE CHECK (email ~ '^[^@\s]+@[^@\s]+\.[^@\s]+$'),
  display_name       text NOT NULL DEFAULT '',
  password_hash      text,
  external_auth_id   text UNIQUE,
  role               text NOT NULL CHECK (role IN ('owner', 'admin', 'reviewer')),
  active             boolean NOT NULL DEFAULT true,
  failed_login_count integer NOT NULL DEFAULT 0 CHECK (failed_login_count >= 0),
  locked_until       timestamptz,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  last_login_at      timestamptz,
  CONSTRAINT admin_users_has_credential CHECK (password_hash IS NOT NULL OR external_auth_id IS NOT NULL)
);
CREATE TRIGGER admin_users_updated_at BEFORE UPDATE ON admin_users
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Server-side sessions. Only a SHA-256 hash of the session token is stored.
CREATE TABLE admin_sessions (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  admin_user_id uuid NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
  token_hash    bytea NOT NULL UNIQUE,
  csrf_token    text NOT NULL,
  ip_hash       text,
  user_agent    text,
  created_at    timestamptz NOT NULL DEFAULT now(),
  last_seen_at  timestamptz NOT NULL DEFAULT now(),
  expires_at    timestamptz NOT NULL,
  revoked_at    timestamptz,
  CONSTRAINT admin_sessions_expiry_after_create CHECK (expires_at > created_at)
);
CREATE INDEX admin_sessions_user_idx ON admin_sessions (admin_user_id) WHERE revoked_at IS NULL;

-- Where data came from and what we are allowed to do with it.
-- Every permission flag defaults to FALSE until evidenced.
CREATE TABLE data_sources (
  id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_type            text NOT NULL CHECK (source_type IN (
                           'self_submission', 'authorized_import', 'official_site',
                           'licensed_feed', 'permitted_api_lookup', 'manual_verification')),
  provider               text NOT NULL CHECK (length(btrim(provider)) > 0),
  source_url             text CHECK (source_url IS NULL OR source_url ~* '^https?://'),
  license_reference      text,
  collected_at           timestamptz NOT NULL DEFAULT now(),
  allowed_internal_use   boolean NOT NULL DEFAULT false,
  allowed_public_display boolean NOT NULL DEFAULT false,
  allowed_commercial_api boolean NOT NULL DEFAULT false,
  retention_expires_at   timestamptz,
  restrictions           jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(restrictions) = 'object'),
  notes                  text,
  declared_by            uuid REFERENCES admin_users(id),
  created_at             timestamptz NOT NULL DEFAULT now(),
  updated_at             timestamptz NOT NULL DEFAULT now(),
  -- Display or redistribution rights are meaningless without internal-use rights.
  CONSTRAINT data_sources_public_requires_internal CHECK (NOT allowed_public_display OR allowed_internal_use),
  CONSTRAINT data_sources_commercial_requires_internal CHECK (NOT allowed_commercial_api OR allowed_internal_use),
  -- Content obtained through third-party discovery APIs (e.g. Google Places) is never
  -- eligible for commercial redistribution or public display through this system.
  CONSTRAINT data_sources_api_lookup_not_redistributable CHECK (
    source_type <> 'permitted_api_lookup' OR (NOT allowed_commercial_api AND NOT allowed_public_display))
);
CREATE INDEX data_sources_permissions_idx
  ON data_sources (allowed_internal_use, allowed_public_display, allowed_commercial_api);
CREATE INDEX data_sources_retention_idx ON data_sources (retention_expires_at) WHERE retention_expires_at IS NOT NULL;
CREATE TRIGGER data_sources_updated_at BEFORE UPDATE ON data_sources
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Business roles (many-to-many with companies). A lookup table rather than a
-- Postgres enum so roles can be added without a type migration.
CREATE TABLE business_roles (
  code        text PRIMARY KEY CHECK (code ~ '^[a-z][a-z_]*$'),
  name        text NOT NULL,
  description text NOT NULL DEFAULT '',
  sort_order  integer NOT NULL DEFAULT 0
);

-- Arbitrary-depth category tree.
CREATE TABLE categories (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  parent_id     uuid REFERENCES categories(id) ON DELETE RESTRICT,
  name          text NOT NULL CHECK (length(btrim(name)) > 0),
  slug          text NOT NULL CHECK (is_valid_slug(slug)),
  category_type text NOT NULL DEFAULT 'industry'
                CHECK (category_type IN ('industry', 'product_category', 'service', 'trade')),
  description   text NOT NULL DEFAULT '',
  -- Official NIC code, only populated after verifying against the official MoSPI publication.
  nic_code      text CHECK (nic_code IS NULL OR nic_code ~ '^[0-9]{2,5}$'),
  nic_version   text CHECK (nic_version IS NULL OR nic_version IN ('NIC2008', 'NIC2025')),
  active        boolean NOT NULL DEFAULT true,
  sort_order    integer NOT NULL DEFAULT 0,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT categories_not_own_parent CHECK (parent_id IS NULL OR parent_id <> id),
  CONSTRAINT categories_nic_pair CHECK ((nic_code IS NULL) = (nic_version IS NULL))
);
-- Slugs are globally unique (they are used in public URLs), which also implies sibling uniqueness.
CREATE UNIQUE INDEX categories_slug_key ON categories (slug);
CREATE UNIQUE INDEX categories_sibling_name_key ON categories (COALESCE(parent_id, '00000000-0000-0000-0000-000000000000'::uuid), lower(name));
CREATE INDEX categories_parent_idx ON categories (parent_id);
CREATE TRIGGER categories_updated_at BEFORE UPDATE ON categories
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Prevent cycles in the category tree.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION categories_prevent_cycle() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.parent_id IS NULL THEN
    RETURN NEW;
  END IF;
  IF EXISTS (
    WITH RECURSIVE ancestors AS (
      SELECT id, parent_id FROM categories WHERE id = NEW.parent_id
      UNION ALL
      SELECT c.id, c.parent_id FROM categories c JOIN ancestors a ON c.id = a.parent_id
    )
    SELECT 1 FROM ancestors WHERE id = NEW.id
  ) THEN
    RAISE EXCEPTION 'category % cannot be its own ancestor', NEW.id USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER categories_cycle_guard BEFORE INSERT OR UPDATE OF parent_id ON categories
  FOR EACH ROW EXECUTE FUNCTION categories_prevent_cycle();

CREATE TABLE capabilities (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug            text NOT NULL UNIQUE CHECK (is_valid_slug(slug)),
  name            text NOT NULL,
  capability_type text NOT NULL CHECK (capability_type IN ('process', 'equipment', 'material', 'service', 'quality', 'capacity')),
  description     text NOT NULL DEFAULT '',
  active          boolean NOT NULL DEFAULT true,
  created_at      timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS capabilities;
DROP TRIGGER IF EXISTS categories_cycle_guard ON categories;
DROP FUNCTION IF EXISTS categories_prevent_cycle();
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS business_roles;
DROP TABLE IF EXISTS data_sources;
DROP TABLE IF EXISTS admin_sessions;
DROP TABLE IF EXISTS admin_users;
DROP TABLE IF EXISTS indian_states;
