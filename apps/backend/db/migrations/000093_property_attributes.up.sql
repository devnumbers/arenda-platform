-- Property attributes: per-type typed characteristics stored as JSONB.
-- Only filled keys are stored; null values are never written. "No attributes"
-- is an empty object '{}'. Validation lives in the application catalog, not in
-- the database (no CHECK constraints on keys). See ADR/issue #124, #128.
ALTER TABLE properties ADD COLUMN attributes JSONB NOT NULL DEFAULT '{}'::jsonb;
