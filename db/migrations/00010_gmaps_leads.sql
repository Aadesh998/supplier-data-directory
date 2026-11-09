-- +goose Up

-- Jobs with provider 'gmaps_scraper' are run by the gmaps-worker command (Google Maps scraper service).
ALTER TABLE discovery_jobs DROP CONSTRAINT discovery_jobs_provider_check;
ALTER TABLE discovery_jobs ADD CONSTRAINT discovery_jobs_provider_check
  CHECK (provider IN ('osm_overpass', 'google_places_ids', 'gmaps_scraper'));

-- Leads found by the gmaps-worker. Raw leads for admin review; never shown publicly as-is.
CREATE TABLE gmaps_leads (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  place_key        text NOT NULL UNIQUE CHECK (length(place_key) BETWEEN 1 AND 512), -- place_id, else cid, else maps link
  first_job_id     uuid REFERENCES discovery_jobs(id) ON DELETE SET NULL,
  last_job_id      uuid REFERENCES discovery_jobs(id) ON DELETE SET NULL,
  category_slug    text,
  state_code       char(2),
  location         text,
  title            text NOT NULL,
  category         text,
  address          text,
  phone            text,
  website          text,
  emails           text,
  latitude         numeric(9, 6),
  longitude        numeric(9, 6),
  review_count     integer,
  review_rating    numeric(3, 2),
  business_status  text,
  maps_link        text,
  complete_address jsonb,
  status           text NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'promoted', 'contacted', 'dismissed')),
  seen_count       integer NOT NULL DEFAULT 1,
  first_seen_at    timestamptz NOT NULL DEFAULT now(),
  last_seen_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX gmaps_leads_status_idx ON gmaps_leads (status, last_seen_at DESC);
CREATE INDEX gmaps_leads_category_idx ON gmaps_leads (category_slug, state_code);

-- +goose Down
DROP TABLE IF EXISTS gmaps_leads;
DELETE FROM discovery_jobs WHERE provider = 'gmaps_scraper';
ALTER TABLE discovery_jobs DROP CONSTRAINT discovery_jobs_provider_check;
ALTER TABLE discovery_jobs ADD CONSTRAINT discovery_jobs_provider_check
  CHECK (provider IN ('osm_overpass', 'google_places_ids'));
