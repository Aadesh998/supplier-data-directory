-- +goose Up
-- Core extensions. gen_random_uuid() is built into PostgreSQL 13+.
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS unaccent;
CREATE EXTENSION IF NOT EXISTS citext;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- Slugs: lowercase ascii words joined by single hyphens.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION is_valid_slug(s text) RETURNS boolean
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
  SELECT s ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND length(s) BETWEEN 1 AND 160
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS is_valid_slug(text);
DROP FUNCTION IF EXISTS set_updated_at();
-- Extensions are left installed deliberately; other databases objects may depend on them.
