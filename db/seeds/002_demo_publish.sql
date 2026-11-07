-- DEVELOPMENT-ONLY DEMO. Publishes the synthetic records so public pages can be previewed locally.
-- The seed runner refuses to apply this file when APP_ENV=production. Public pages label these
-- records as synthetic because their names contain "(Synthetic)".
UPDATE data_sources SET allowed_public_display = true
WHERE provider = 'synthetic-seed';

UPDATE companies c
SET publication_status = 'published', published_at = COALESCE(published_at, now())
WHERE c.deleted_at IS NULL
  AND c.publication_status = 'approved_private'
  AND EXISTS (SELECT 1 FROM field_provenance fp JOIN data_sources ds ON ds.id = fp.source_id
              WHERE fp.company_id = c.id AND ds.provider = 'synthetic-seed');
