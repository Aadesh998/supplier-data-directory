-- +goose Up

-- Append-only audit log. Callers must redact secrets/personal data before writing.
CREATE TABLE audit_logs (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_id    uuid REFERENCES admin_users(id),
  actor_type  text NOT NULL DEFAULT 'admin' CHECK (actor_type IN ('admin', 'system', 'public')),
  action      text NOT NULL CHECK (action ~ '^[a-z][a-z0-9_.]*$'),
  entity_type text NOT NULL,
  entity_id   uuid,
  before      jsonb,
  after       jsonb,
  request_id  text,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_entity_idx ON audit_logs (entity_type, entity_id, created_at DESC);
CREATE INDEX audit_logs_created_idx ON audit_logs (created_at DESC);
CREATE INDEX audit_logs_actor_idx ON audit_logs (actor_id, created_at DESC);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION audit_logs_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'audit_logs is append-only' USING ERRCODE = 'insufficient_privilege';
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER audit_logs_no_update BEFORE UPDATE OR DELETE ON audit_logs
  FOR EACH ROW EXECUTE FUNCTION audit_logs_append_only();

-- Future external API (M8). Disabled for external consumers in v1.
CREATE TABLE api_clients (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name       text NOT NULL,
  status     text NOT NULL DEFAULT 'disabled' CHECK (status IN ('disabled', 'active', 'suspended')),
  plan       text,
  notes      text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  -- v1 guard: no external client may be activated. Drop this constraint in the M8 migration
  -- only after legal review of commercial redistribution rights.
  CONSTRAINT api_clients_disabled_in_v1 CHECK (status <> 'active')
);
CREATE TRIGGER api_clients_updated_at BEFORE UPDATE ON api_clients
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE api_keys (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  client_id    uuid NOT NULL REFERENCES api_clients(id) ON DELETE CASCADE,
  key_prefix   text NOT NULL CHECK (length(key_prefix) BETWEEN 6 AND 16),
  key_hash     bytea NOT NULL UNIQUE,
  scopes       text[] NOT NULL DEFAULT '{}',
  created_at   timestamptz NOT NULL DEFAULT now(),
  expires_at   timestamptz,
  revoked_at   timestamptz,
  last_used_at timestamptz
);
CREATE INDEX api_keys_client_idx ON api_keys (client_id);

CREATE TABLE api_usage_daily (
  client_id      uuid NOT NULL REFERENCES api_clients(id) ON DELETE CASCADE,
  usage_date     date NOT NULL,
  endpoint_group text NOT NULL,
  request_count  bigint NOT NULL DEFAULT 0 CHECK (request_count >= 0),
  error_count    bigint NOT NULL DEFAULT 0 CHECK (error_count >= 0),
  PRIMARY KEY (client_id, usage_date, endpoint_group)
);

-- No DISTINCT: callers use EXISTS or dedupe, and DISTINCT would block predicate pushdown.
-- Fields that may be shown publicly: permitted provenance from a non-expired source that allows public display.
CREATE VIEW public_field_permissions AS
SELECT fp.company_id, fp.entity_type, fp.entity_id, fp.field_name
FROM field_provenance fp
JOIN data_sources ds ON ds.id = fp.source_id
WHERE fp.permission_status = 'permitted'
  AND ds.allowed_public_display
  AND (ds.retention_expires_at IS NULL OR ds.retention_expires_at > now());

-- Fields that may be redistributed through a commercial API / export.
CREATE VIEW commercial_field_permissions AS
SELECT fp.company_id, fp.entity_type, fp.entity_id, fp.field_name
FROM field_provenance fp
JOIN data_sources ds ON ds.id = fp.source_id
WHERE fp.permission_status = 'permitted'
  AND ds.allowed_commercial_api
  AND (ds.retention_expires_at IS NULL OR ds.retention_expires_at > now());

-- Fields that may be used internally (admin console, internal exports).
CREATE VIEW internal_field_permissions AS
SELECT fp.company_id, fp.entity_type, fp.entity_id, fp.field_name
FROM field_provenance fp
JOIN data_sources ds ON ds.id = fp.source_id
WHERE fp.permission_status IN ('permitted', 'unknown')
  AND ds.allowed_internal_use
  AND (ds.retention_expires_at IS NULL OR ds.retention_expires_at > now());

-- +goose Down
DROP VIEW IF EXISTS internal_field_permissions;
DROP VIEW IF EXISTS commercial_field_permissions;
DROP VIEW IF EXISTS public_field_permissions;
DROP TABLE IF EXISTS api_usage_daily;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS api_clients;
DROP TRIGGER IF EXISTS audit_logs_no_update ON audit_logs;
DROP FUNCTION IF EXISTS audit_logs_append_only();
DROP TABLE IF EXISTS audit_logs;
