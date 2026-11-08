-- +goose Up

-- Each CSV upload of discovery jobs.
CREATE TABLE discovery_job_batches (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  filename    text NOT NULL,
  file_hash   text NOT NULL CHECK (file_hash ~ '^[0-9a-f]{64}$'),
  inserted    integer NOT NULL DEFAULT 0,
  updated     integer NOT NULL DEFAULT 0,
  skipped     integer NOT NULL DEFAULT 0,
  errors      jsonb NOT NULL DEFAULT '[]'::jsonb,
  uploaded_by uuid NOT NULL REFERENCES admin_users(id),
  created_at  timestamptz NOT NULL DEFAULT now()
);

-- Discovery jobs: one search (sector keyword x location). Workers claim jobs with a lease;
-- completed jobs become due again after refresh_interval_days so the database keeps updating.
CREATE TABLE discovery_jobs (
  id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  job_key               text NOT NULL UNIQUE CHECK (length(job_key) BETWEEN 1 AND 64),
  batch_id              uuid REFERENCES discovery_job_batches(id) ON DELETE SET NULL,
  provider              text NOT NULL DEFAULT 'osm_overpass'
                        CHECK (provider IN ('osm_overpass', 'google_places_ids')),
  priority              smallint NOT NULL DEFAULT 3 CHECK (priority BETWEEN 1 AND 9),
  sector_family         text,
  category_slug         text REFERENCES categories(slug) ON UPDATE CASCADE,
  keyword               text NOT NULL,
  search_query          text NOT NULL,
  osm_name_regex        text,
  location              text NOT NULL,
  state_code            char(2) REFERENCES indian_states(code),
  latitude              numeric(9, 6) NOT NULL CHECK (latitude BETWEEN -90 AND 90),
  longitude             numeric(9, 6) NOT NULL CHECK (longitude BETWEEN -180 AND 180),
  radius_km             numeric(6, 2) NOT NULL CHECK (radius_km > 0 AND radius_km <= 50),
  location_tier         smallint,
  is_cluster            boolean NOT NULL DEFAULT false,
  status                text NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending', 'running', 'done', 'failed', 'paused')),
  attempts              integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  max_attempts          integer NOT NULL DEFAULT 3 CHECK (max_attempts BETWEEN 1 AND 20),
  refresh_interval_days integer NOT NULL DEFAULT 30 CHECK (refresh_interval_days BETWEEN 1 AND 365),
  locked_by             text,
  lease_expires_at      timestamptz,
  last_run_at           timestamptz,
  next_run_after        timestamptz,
  last_error            text,
  results_count         integer,
  total_results         integer NOT NULL DEFAULT 0,
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT discovery_jobs_running_has_lease CHECK (status <> 'running' OR (locked_by IS NOT NULL AND lease_expires_at IS NOT NULL))
);
-- The claim query: due jobs by priority.
CREATE INDEX discovery_jobs_due_idx ON discovery_jobs (priority, next_run_after NULLS FIRST, created_at)
  WHERE status IN ('pending', 'done');
CREATE INDEX discovery_jobs_running_idx ON discovery_jobs (lease_expires_at) WHERE status = 'running';
CREATE INDEX discovery_jobs_status_idx ON discovery_jobs (status, priority);
CREATE TRIGGER discovery_jobs_updated_at BEFORE UPDATE ON discovery_jobs
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- What workers found. A result is a lead for review, never directly a public record.
CREATE TABLE discovery_results (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  provider           text NOT NULL CHECK (provider IN ('osm_overpass', 'google_places_ids')),
  provider_ref       text NOT NULL CHECK (length(provider_ref) BETWEEN 1 AND 512),
  first_job_id       uuid REFERENCES discovery_jobs(id) ON DELETE SET NULL,
  last_job_id        uuid REFERENCES discovery_jobs(id) ON DELETE SET NULL,
  category_slug      text,
  name               text,
  latitude           numeric(9, 6),
  longitude          numeric(9, 6),
  tags               jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(tags) = 'object'),
  status             text NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'promoted', 'contacted', 'dismissed')),
  matched_company_id uuid REFERENCES companies(id) ON DELETE SET NULL,
  seen_count         integer NOT NULL DEFAULT 1,
  first_seen_at      timestamptz NOT NULL DEFAULT now(),
  last_seen_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, provider_ref),
  -- Google permits storing place IDs only; names, coordinates and other content may not be kept.
  CONSTRAINT discovery_results_google_ids_only CHECK (
    provider <> 'google_places_ids' OR (name IS NULL AND latitude IS NULL AND longitude IS NULL AND tags = '{}'::jsonb))
);
CREATE INDEX discovery_results_status_idx ON discovery_results (status, last_seen_at DESC);
CREATE INDEX discovery_results_category_idx ON discovery_results (category_slug);

-- +goose Down
DROP TABLE IF EXISTS discovery_results;
DROP TABLE IF EXISTS discovery_jobs;
DROP TABLE IF EXISTS discovery_job_batches;
