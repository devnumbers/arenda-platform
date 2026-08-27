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

-- Платежи объекта для экрана «Платежи объекта» (тикет #463): два обычных
-- правила и автоплатёж на квартире; гараж намеренно пуст — экран показывает
-- пустые состояния. `since` в будущем: загрузочный тик бекенда ничего не
-- материализует, и в секции «Просроченные» — ровно сидовые операции ниже. Слаги категорий — из дефолтного каталога
-- (tools/payment-categories/catalog.json), иконку рисует фронт.
INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks,
                      recurrence, since, end_date, auto_pay, payment_form,
                      category_slug, is_favorite)
VALUES
    ('55555555-5555-4555-8555-555555555551',
     '11111111-1111-4111-8111-111111111111',
     '33333333-3333-4333-8333-333333333333',
     'expense', 'Арендная плата', 4500000,
     '{"kind": "monthly", "dayOfMonth": 1}',
     CURRENT_DATE + 5, NULL, FALSE, 'transfer', 'rent', TRUE),
    ('55555555-5555-4555-8555-555555555552',
     '11111111-1111-4111-8111-111111111111',
     '33333333-3333-4333-8333-333333333333',
     'expense', 'Страхование', 320000,
     '{"kind": "monthly", "dayOfMonth": 15}',
     CURRENT_DATE + 5, NULL, FALSE, 'cash', 'insurance', FALSE),
    ('55555555-5555-4555-8555-555555555553',
     '11111111-1111-4111-8111-111111111111',
     '33333333-3333-4333-8333-333333333333',
     'expense', 'Электроэнергия', 120000,
     '{"kind": "monthly", "dayOfMonth": 5}',
     CURRENT_DATE + 5, NULL, TRUE, 'transfer', 'electricity', FALSE),
-- Страница платежа (#465): правило на активной бессрочной паузе
-- («Возобновить», «Ближайший платеж» = «На паузе») и завершённое правило
-- (endDate в прошлом: без паузы, «Оплатить» отключена).
    ('55555555-5555-4555-8555-555555555554',
     '11111111-1111-4111-8111-111111111111',
     '33333333-3333-4333-8333-333333333333',
     'expense', 'Домофон', 15000,
     '{"kind": "monthly", "dayOfMonth": 10}',
     CURRENT_DATE - 60, NULL, FALSE, 'transfer', 'intercom', FALSE),
    ('55555555-5555-4555-8555-555555555555',
     '11111111-1111-4111-8111-111111111111',
     '33333333-3333-4333-8333-333333333333',
     'expense', 'Техосмотр', 90000,
     '{"kind": "yearly", "month": 2, "day": 29}',
     CURRENT_DATE - 14, CURRENT_DATE - 14, FALSE, 'cash', 'parking', FALSE)
ON CONFLICT (id) DO UPDATE
SET title = EXCLUDED.title,
    amount_kopecks = EXCLUDED.amount_kopecks,
    recurrence = EXCLUDED.recurrence,
    since = EXCLUDED.since,
    end_date = EXCLUDED.end_date,
    auto_pay = EXCLUDED.auto_pay,
    payment_form = EXCLUDED.payment_form,
    category_slug = EXCLUDED.category_slug,
    is_favorite = EXCLUDED.is_favorite;

-- Активная бессрочная пауза «Домофона»: from в прошлом, to не задан.
INSERT INTO payment_pauses (id, payment_id, from_date, to_date)
VALUES ('88888888-8888-4888-8888-888888888881',
        '55555555-5555-4555-8555-555555555554',
        CURRENT_DATE - 3, NULL)
ON CONFLICT (id) DO UPDATE
SET from_date = EXCLUDED.from_date,
    to_date = EXCLUDED.to_date;

-- Просроченные вхождения: planned с прошедшей датой — просрочку проецирует
-- сервер по «сегодня» в TZ собственника (ADR 0048), хранится статус planned.
INSERT INTO operations (id, owner_id, property_id, payment_id, origin, date,
                        paid_date, status, type, title, amount_kopecks,
                        payment_form, category_label, category_slug)
VALUES
    ('77777777-7777-4777-8777-777777777771',
     '11111111-1111-4111-8111-111111111111',
     '33333333-3333-4333-8333-333333333333',
     '55555555-5555-4555-8555-555555555551',
     'payment', CURRENT_DATE - 5, NULL, 'planned', 'expense',
     'Арендная плата', 4500000, 'transfer', 'Арендная плата', 'rent'),
    ('77777777-7777-4777-8777-777777777772',
     '11111111-1111-4111-8111-111111111111',
     '33333333-3333-4333-8333-333333333333',
     '55555555-5555-4555-8555-555555555552',
     'payment', CURRENT_DATE - 2, NULL, 'planned', 'expense',
     'Страхование', 320000, 'cash', 'Страхование', 'insurance')
ON CONFLICT (id) DO UPDATE
SET date = EXCLUDED.date,
    paid_date = EXCLUDED.paid_date,
    status = EXCLUDED.status;
