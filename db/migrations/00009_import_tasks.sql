-- +goose Up
-- Long-running import work (row processing, dry run, commit) runs as a background task so that
-- large files are not cut off by HTTP/proxy timeouts. One task per import at a time.
ALTER TABLE import_jobs
  ADD COLUMN task text CHECK (task IN ('process', 'dry_run', 'commit')),
  ADD COLUMN task_started_at timestamptz,
  ADD COLUMN task_finished_at timestamptz,
  ADD COLUMN task_result jsonb,
  ADD COLUMN task_error text,
  ADD CONSTRAINT import_jobs_task_started CHECK (task IS NULL OR task_started_at IS NOT NULL);

-- +goose Down
ALTER TABLE import_jobs
  DROP CONSTRAINT IF EXISTS import_jobs_task_started,
  DROP COLUMN IF EXISTS task_error,
  DROP COLUMN IF EXISTS task_result,
  DROP COLUMN IF EXISTS task_finished_at,
  DROP COLUMN IF EXISTS task_started_at,
  DROP COLUMN IF EXISTS task;
