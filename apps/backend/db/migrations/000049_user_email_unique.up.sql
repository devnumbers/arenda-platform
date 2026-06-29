-- Normalize existing emails so uniqueness is case-insensitive.
UPDATE users
SET email = LOWER(email)
WHERE email IS NOT NULL;

-- Resolve duplicate emails by clearing the address on all but the oldest user.
-- The kept row is determined by created_at (and id as a tie-breaker).
WITH ranked AS (
  SELECT id,
         ROW_NUMBER() OVER (PARTITION BY LOWER(email) ORDER BY created_at ASC, id ASC) AS rn
  FROM users
  WHERE email IS NOT NULL
)
UPDATE users u
SET email = NULL
FROM ranked r
WHERE u.id = r.id AND r.rn > 1;

-- Enforce uniqueness at the database level using a case-insensitive index.
CREATE UNIQUE INDEX users_email_lowercase_unique
ON users (LOWER(email));