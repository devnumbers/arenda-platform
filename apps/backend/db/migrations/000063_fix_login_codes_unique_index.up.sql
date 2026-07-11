-- Deduplicate unused codes with NULL email, keeping the newest per (phone, purpose).
WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY phone, purpose
                              ORDER BY created_at DESC, id DESC) AS rn
    FROM login_codes
    WHERE used = false
      AND email IS NULL
)
DELETE FROM login_codes
WHERE id IN (SELECT id FROM ranked WHERE rn > 1);

DROP INDEX IF EXISTS idx_login_codes_unique_unused;

CREATE UNIQUE INDEX IF NOT EXISTS idx_login_codes_unique_unused
ON login_codes (phone, COALESCE(email, ''), purpose)
WHERE used = false;
