-- Remove expired unused codes before enforcing uniqueness.
-- These rows are no longer valid and would only block the index creation.
DELETE FROM login_codes
WHERE used = false
  AND expires_at <= now();

-- Deduplicate unused codes, keeping the newest one per (phone, email, purpose).
WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY phone, email, purpose
                              ORDER BY created_at DESC, id DESC) AS rn
    FROM login_codes
    WHERE used = false
)
DELETE FROM login_codes
WHERE id IN (SELECT id FROM ranked WHERE rn > 1);

CREATE UNIQUE INDEX IF NOT EXISTS idx_login_codes_unique_unused
ON login_codes (phone, email, purpose)
WHERE used = false;
