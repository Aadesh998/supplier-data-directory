-- +goose Up

-- Supplier self-registrations. Remain pending until a human reviews them.
CREATE TABLE supplier_submissions (
  id                                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  status                             text NOT NULL DEFAULT 'submitted'
                                     CHECK (status IN ('submitted', 'under_review', 'approved', 'rejected',
                                                       'needs_changes', 'spam', 'withdrawn')),
  payload                            jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
  -- Submitter email is encrypted at rest; the HMAC allows look-ups without decryption.
  submitter_email_encrypted          bytea NOT NULL,
  submitter_email_hash               text NOT NULL,
  submitter_ip_hash                  text,
  privacy_notice_version             text NOT NULL CHECK (length(privacy_notice_version) > 0),
  -- Three separate, never pre-checked authorisations.
  storage_consent_at                 timestamptz NOT NULL,
  public_listing_permission_at       timestamptz,
  future_commercial_api_permission_at timestamptz,
  public_listing_withdrawn_at        timestamptz,
  commercial_api_withdrawn_at        timestamptz,
  idempotency_key                    text UNIQUE CHECK (idempotency_key IS NULL OR length(idempotency_key) BETWEEN 8 AND 128),
  submitted_at                       timestamptz NOT NULL DEFAULT now(),
  reviewed_at                        timestamptz,
  reviewer_id                        uuid REFERENCES admin_users(id),
  review_notes                       text,
  company_id                         uuid REFERENCES companies(id) ON DELETE SET NULL,
  source_id                          uuid REFERENCES data_sources(id),
  created_at                         timestamptz NOT NULL DEFAULT now(),
  updated_at                         timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT submissions_review_recorded CHECK (
    status NOT IN ('approved', 'rejected', 'needs_changes', 'spam') OR (reviewed_at IS NOT NULL AND reviewer_id IS NOT NULL)),
  CONSTRAINT submissions_approved_has_company CHECK (status <> 'approved' OR company_id IS NOT NULL)
);
CREATE INDEX supplier_submissions_status_idx ON supplier_submissions (status, submitted_at DESC);
CREATE INDEX supplier_submissions_email_hash_idx ON supplier_submissions (submitter_email_hash);
CREATE TRIGGER supplier_submissions_updated_at BEFORE UPDATE ON supplier_submissions
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE documents ADD CONSTRAINT documents_submission_fk
  FOREIGN KEY (submission_id) REFERENCES supplier_submissions(id) ON DELETE CASCADE;

CREATE TABLE import_jobs (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  filename           text NOT NULL,
  file_hash          text NOT NULL CHECK (file_hash ~ '^[0-9a-f]{64}$'),
  file_size          bigint NOT NULL CHECK (file_size >= 0),
  declared_source_id uuid NOT NULL REFERENCES data_sources(id),
  status             text NOT NULL DEFAULT 'uploaded'
                     CHECK (status IN ('uploaded', 'staged', 'committed', 'failed', 'rolled_back', 'cancelled')),
  headers            jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(headers) = 'array'),
  column_mapping     jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(column_mapping) = 'object'),
  total_rows         integer NOT NULL DEFAULT 0,
  valid_rows         integer NOT NULL DEFAULT 0,
  error_rows         integer NOT NULL DEFAULT 0,
  duplicate_rows     integer NOT NULL DEFAULT 0,
  committed_rows     integer NOT NULL DEFAULT 0,
  errors             jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(errors) = 'array'),
  initiated_by       uuid NOT NULL REFERENCES admin_users(id),
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  committed_at       timestamptz,
  finished_at        timestamptz,
  rolled_back_at     timestamptz
);
-- Idempotency: the same file for the same declared source can only be active once.
CREATE UNIQUE INDEX import_jobs_active_file_key ON import_jobs (file_hash, declared_source_id)
  WHERE status IN ('uploaded', 'staged', 'committed');
CREATE INDEX import_jobs_status_idx ON import_jobs (status, created_at DESC);
CREATE TRIGGER import_jobs_updated_at BEFORE UPDATE ON import_jobs
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE import_rows (
  id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  import_job_id        uuid NOT NULL REFERENCES import_jobs(id) ON DELETE CASCADE,
  row_number           integer NOT NULL CHECK (row_number > 0),
  raw_data             jsonb NOT NULL CHECK (jsonb_typeof(raw_data) = 'object'),
  normalized_data      jsonb NOT NULL DEFAULT '{}'::jsonb,
  transformation_log   jsonb NOT NULL DEFAULT '[]'::jsonb,
  duplicate_candidates jsonb NOT NULL DEFAULT '[]'::jsonb,
  validation_errors    jsonb NOT NULL DEFAULT '[]'::jsonb,
  validation_warnings  jsonb NOT NULL DEFAULT '[]'::jsonb,
  disposition          text NOT NULL DEFAULT 'pending'
                       CHECK (disposition IN ('pending', 'accepted', 'rejected', 'merged')),
  matched_company_id   uuid REFERENCES companies(id) ON DELETE SET NULL,
  created_company_id   uuid REFERENCES companies(id) ON DELETE SET NULL,
  reviewed_by          uuid REFERENCES admin_users(id),
  committed_at         timestamptz,
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),
  UNIQUE (import_job_id, row_number),
  CONSTRAINT import_rows_merge_target CHECK (disposition <> 'merged' OR matched_company_id IS NOT NULL)
);
CREATE INDEX import_rows_job_disposition_idx ON import_rows (import_job_id, disposition, row_number);
CREATE TRIGGER import_rows_updated_at BEFORE UPDATE ON import_rows
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Potential duplicate company pairs awaiting human review. Never auto-merged.
CREATE TABLE duplicate_pairs (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_a_id     uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  company_b_id     uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  score            numeric(4, 3) NOT NULL CHECK (score BETWEEN 0 AND 1),
  reasons          jsonb NOT NULL DEFAULT '[]'::jsonb,
  status           text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'merged', 'dismissed')),
  resolved_by      uuid REFERENCES admin_users(id),
  resolved_at      timestamptz,
  resolution_notes text,
  created_at       timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT duplicate_pairs_ordered CHECK (company_a_id < company_b_id),
  CONSTRAINT duplicate_pairs_resolution CHECK (status = 'open' OR (resolved_by IS NOT NULL AND resolved_at IS NOT NULL)),
  UNIQUE (company_a_id, company_b_id)
);
CREATE INDEX duplicate_pairs_status_idx ON duplicate_pairs (status, score DESC);

-- Privacy requests (correction, removal, consent withdrawal). Table name kept from the spec.
CREATE TABLE deletion_requests (
  id                        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  request_type              text NOT NULL CHECK (request_type IN (
                              'correction', 'removal', 'withdraw_public_listing', 'withdraw_commercial_api', 'access', 'other')),
  requester                 text NOT NULL CHECK (length(btrim(requester)) BETWEEN 1 AND 200),
  requester_email_encrypted bytea NOT NULL,
  requester_email_hash      text NOT NULL,
  company_id                uuid REFERENCES companies(id) ON DELETE SET NULL,
  company_reference         text,
  scope                     text NOT NULL DEFAULT 'all',
  details                   text CHECK (details IS NULL OR length(details) <= 5000),
  status                    text NOT NULL DEFAULT 'received'
                            CHECK (status IN ('received', 'in_progress', 'resolved', 'rejected')),
  received_at               timestamptz NOT NULL DEFAULT now(),
  acknowledged_at           timestamptz,
  resolved_at               timestamptz,
  resolved_by               uuid REFERENCES admin_users(id),
  resolution_notes          text,
  updated_at                timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT deletion_requests_resolution CHECK (status NOT IN ('resolved', 'rejected') OR resolved_at IS NOT NULL)
);
CREATE INDEX deletion_requests_status_idx ON deletion_requests (status, received_at);
CREATE TRIGGER deletion_requests_updated_at BEFORE UPDATE ON deletion_requests
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Discovery leads: ONLY provider identifiers (e.g. Google place IDs, which Google permits
-- storing) plus our own notes. No provider content (names, addresses, ratings) is persisted.
CREATE TABLE discovery_leads (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  provider          text NOT NULL CHECK (provider IN ('google_places')),
  provider_place_id text NOT NULL CHECK (length(provider_place_id) BETWEEN 1 AND 512),
  search_query      text,
  status            text NOT NULL DEFAULT 'new'
                    CHECK (status IN ('new', 'invited', 'registered', 'dismissed')),
  notes             text,
  created_by        uuid NOT NULL REFERENCES admin_users(id),
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, provider_place_id)
);
CREATE TRIGGER discovery_leads_updated_at BEFORE UPDATE ON discovery_leads
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS discovery_leads;
DROP TABLE IF EXISTS deletion_requests;
DROP TABLE IF EXISTS duplicate_pairs;
DROP TABLE IF EXISTS import_rows;
DROP TABLE IF EXISTS import_jobs;
ALTER TABLE documents DROP CONSTRAINT IF EXISTS documents_submission_fk;
DROP TABLE IF EXISTS supplier_submissions;
