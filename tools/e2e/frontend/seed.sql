-- Deterministic seed for the frontend Playwright e2e stack (ticket #456).
-- Applied by tools/e2e/frontend/run-frontend-e2e.sh against the disposable
-- arenda-e2e database right after migrations. SQL-seeding precedent:
-- tools/e2e/sql of the Bruno API-e2e.
--
-- Ids are fixed so the seed is reproducible; :token_hash is the per-run
-- HMAC-SHA256 of the raw session token the orchestrator hands to Playwright
-- (E2E_SESSION_TOKEN), and :phone_det is the deterministic phone ciphertext
-- (users.phone is searched by DeterministicEncrypt output, not plaintext —
-- both come from e2e-crypto.mjs with the same ENCRYPTION_KEY the backend
-- runs with). Statements are idempotent (ON CONFLICT DO NOTHING / UPDATE)
-- so re-seeding a non-reset database does not fail.
--
-- The user has a known phone + email, so the /login UI goes straight from
-- the phone step to the code step (the code lands in the backend log via
-- the fake email sender), and the pre-authenticated session row lets screen
-- tests skip the login flow entirely.

INSERT INTO users (id, phone, role, name, surname, email, phone_encrypted)
VALUES ('11111111-1111-4111-8111-111111111111', :'phone_det', 'owner', 'Иван', 'Иванов', 'e2e@example.com', TRUE)
ON CONFLICT (id) DO UPDATE
SET phone = EXCLUDED.phone,
    phone_encrypted = TRUE,
    email = EXCLUDED.email,
    name = EXCLUDED.name,
    surname = EXCLUDED.surname;

INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at, last_used_at)
VALUES ('22222222-2222-4222-8222-222222222222',
        '11111111-1111-4111-8111-111111111111',
        :'token_hash',
        now() + interval '7 days',
        now(),
        now())
ON CONFLICT (id) DO UPDATE
SET token_hash = EXCLUDED.token_hash,
    expires_at = EXCLUDED.expires_at,
    last_used_at = EXCLUDED.last_used_at;

INSERT INTO properties (id, owner_id, name, type, address, status)
VALUES
    ('33333333-3333-4333-8333-333333333333',
     '11111111-1111-4111-8111-111111111111',
     'Квартира на Ленина', 'apartment', 'Москва, ул. Ленина, 1', 'active'),
    ('44444444-4444-4444-8444-444444444444',
     '11111111-1111-4111-8111-111111111111',
     'Гараж на Садовой', 'garage', 'Москва, ул. Садовая, 2', 'active')
ON CONFLICT (id) DO NOTHING;
