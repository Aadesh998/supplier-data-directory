-- +goose Up
-- GSTIN (Goods and Services Tax Identification Number): 2-digit state code + PAN + entity number +
-- 'Z' + check character. Internal only: never part of public or commercial responses. Used for
-- duplicate detection and to infer the registration state.
ALTER TABLE companies ADD COLUMN gstin text
  CONSTRAINT companies_gstin_format CHECK (gstin IS NULL OR gstin ~ '^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$');
-- One live company per GSTIN (merged/soft-deleted records keep theirs for history).
CREATE UNIQUE INDEX companies_gstin_key ON companies (gstin) WHERE gstin IS NOT NULL AND deleted_at IS NULL AND merged_into_id IS NULL;

-- +goose Down
DROP INDEX IF EXISTS companies_gstin_key;
ALTER TABLE companies DROP COLUMN IF EXISTS gstin;
