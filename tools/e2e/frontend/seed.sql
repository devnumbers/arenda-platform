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

-- Календарь владельца (ADR 0048: «сегодня» сервера = users.timezone) — UTC,
-- в одну линию с timezoneId браузера под Playwright (#796): иначе в
-- полуночном окне 00:00–03:00 МСК валидация «дата не в прошлом» отбивает
-- 400 на создание аренды/платежа из визардов, у которых префилл — «сегодня».
INSERT INTO users (id, phone, role, name, surname, email, phone_encrypted, timezone)
VALUES ('11111111-1111-4111-8111-111111111111', :'phone_det', 'owner', 'Иван', 'Иванов', 'e2e@example.com', TRUE, 'UTC')
ON CONFLICT (id) DO UPDATE
SET phone = EXCLUDED.phone,
    phone_encrypted = TRUE,
    email = EXCLUDED.email,
    name = EXCLUDED.name,
    surname = EXCLUDED.surname,
    timezone = EXCLUDED.timezone;

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
     'Гараж на Садовой', 'garage', 'Москва, ул. Садовая, 2', 'active'),
-- Третий объект — только для полного списка просроченных (#466): 55+
-- просрочек одного правила; держит их подальше от квартиры, чтобы секция
-- «Просроченные» её экрана оставалась двухкарточной (Figma 654:6778).
    ('46464646-4646-4646-8646-464646464646',
     '11111111-1111-4111-8111-111111111111',
     'Студия на Полевой', 'apartment', 'Москва, ул. Полевая, 3', 'active')
ON CONFLICT (id) DO NOTHING;

-- Совместный доступ к квартире (#467): полный доступ (правит без удаления)
-- и смотрящий (экран правки недоступен). Собственные сессии — те же
-- pre-authenticated cookie-токены оркестратора.
INSERT INTO users (id, phone, role, name, surname, email, phone_encrypted, timezone)
VALUES
    ('12111111-1111-4111-8111-111111111121', :'member_phone_det', 'owner', 'Мария', 'Петрова', 'e2e-member@example.com', TRUE, 'UTC'),
    ('13111111-1111-4111-8111-111111111131', :'viewer_phone_det', 'owner', 'Сергей', 'Сидоров', 'e2e-viewer@example.com', TRUE, 'UTC')
ON CONFLICT (id) DO UPDATE
SET phone = EXCLUDED.phone,
    phone_encrypted = TRUE,
    email = EXCLUDED.email,
    name = EXCLUDED.name,
    surname = EXCLUDED.surname,
    timezone = EXCLUDED.timezone;

INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at, last_used_at)
VALUES
    ('22222222-2222-4222-8222-222222222223',
     '12111111-1111-4111-8111-111111111121',
     :'member_token_hash',
     now() + interval '7 days',
     now(),
     now()),
    ('22222222-2222-4222-8222-222222222224',
     '13111111-1111-4111-8111-111111111131',
     :'viewer_token_hash',
     now() + interval '7 days',
     now(),
     now())
ON CONFLICT (id) DO UPDATE
SET token_hash = EXCLUDED.token_hash,
    expires_at = EXCLUDED.expires_at,
    last_used_at = EXCLUDED.last_used_at;

-- Уникальность активных членств — частичный индекс (000095): conflict-target
-- повторяет его условие.
--
-- НЕ БАГ (решение владельца, grill #760): эти АКТИВНЫЕ ноги Марии и Сергея на
-- Квартиру сознательно нарушают инвариант слотов получателя. slot_coordinator
-- применяет лимит ПОЛУЧАТЕЛЯ ноги, а получатели этих ног — сами Мария и
-- Сергей: подписок у них в сиде нет, лимит 0 (спеки suspended/slot-сценариев
-- требуют живых участников). Ивану как владельцу это не угрожает: у него
-- подписка pro (лимит 5, сидится ниже в этом же файле). Enforcement на
-- применённом платеже запускается только при СМЕНЕ тарифа
-- (transitionChangedTariff, payment_service.go): апгрейд владельца суспендит
-- такие ноги — на живых приёмках это выглядит как «участник пропал после
-- чужого апгрейда»; реневал pro→pro ноги не трогает.
INSERT INTO property_members (id, property_id, user_id, role, granted_by)
VALUES
    ('99999999-9999-4999-8999-999999999931',
     '33333333-3333-4333-8333-333333333333',
     '12111111-1111-4111-8111-111111111121',
     'full_access',
     '11111111-1111-4111-8111-111111111111'),
    ('99999999-9999-4999-8999-999999999932',
     '33333333-3333-4333-8333-333333333333',
     '13111111-1111-4111-8111-111111111131',
     'viewer',
     '11111111-1111-4111-8111-111111111111')
ON CONFLICT (property_id, user_id) WHERE status = 'active' DO UPDATE
SET role = EXCLUDED.role,
    granted_by = EXCLUDED.granted_by;

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
     CURRENT_DATE - 14, CURRENT_DATE - 14, FALSE, 'cash', 'parking', FALSE),
-- Подэкраны страницы платежа (#466): длинная история оплат (55+ paid —
-- скролл-догрузка истории; since в будущем, чтобы загрузочный тик ничего
-- не материализовал поверх сидовых операций — тот же приём, что у аренды)
-- и ежедневный платеж с длинной будущей проекцией (график, 3+ порции).
    ('55555555-5555-4555-8555-555555555556',
     '11111111-1111-4111-8111-111111111111',
     '33333333-3333-4333-8333-333333333333',
     'expense', 'Интернет', 100000,
     '{"kind": "monthly", "dayOfMonth": 15}',
     CURRENT_DATE + 5, NULL, FALSE, 'transfer', 'internet', FALSE),
    ('55555555-5555-4555-8555-555555555557',
     '11111111-1111-4111-8111-111111111111',
     '33333333-3333-4333-8333-333333333333',
     'expense', 'Парковка', 20000,
     '{"kind": "daily"}',
     CURRENT_DATE, CURRENT_DATE + 120, FALSE, 'cash', 'parking', FALSE),
-- Полный список просроченных (#466): правило студии с 55 просрочками
-- (скролл-догрузка). `since` = сегодня − 5: загрузочный тик не добавляет
-- задним числом, просрочки — ровно сидовые строки ниже.
    ('55555555-5555-4555-8555-555555555558',
     '11111111-1111-4111-8111-111111111111',
     '46464646-4646-4646-8646-464646464646',
     'expense', 'Аренда студии', 3000000,
     '{"kind": "monthly", "dayOfMonth": 10}',
     CURRENT_DATE - 5, NULL, FALSE, 'transfer', 'rent', FALSE),
-- Экран правки и удаление (#467): пара правил-однодневок на студии (в стороне
-- от секций квартиры) с просрочками для обоих режимов чекбокса модалки:
-- «Консьерж-сервис» удаляют без чекбокса (долг остаётся), «Телевидение» —
-- с чекбоксом (просрочки сносятся). `since` в будущем: тик ничего не
-- материализует, просрочки — ровно сидовые строки ниже.
    ('55555555-5555-4555-8555-555555555559',
     '11111111-1111-4111-8111-111111111111',
     '46464646-4646-4646-8646-464646464646',
     'expense', 'Консьерж-сервис', 50000,
     '{"kind": "monthly", "dayOfMonth": 20}',
     CURRENT_DATE + 5, NULL, FALSE, 'transfer', 'concierge', FALSE),
    ('55555555-5555-4555-8555-55555555555a',
     '11111111-1111-4111-8111-111111111111',
     '46464646-4646-4646-8646-464646464646',
     'expense', 'Телевидение', 70000,
     '{"kind": "monthly", "dayOfMonth": 25}',
     CURRENT_DATE + 5, NULL, FALSE, 'transfer', 'tv', FALSE)
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
     'Страхование', 320000, 'cash', 'Страхование', 'insurance'),
-- Просрочки правил под удаление (#467): по паре на «Консьерж-сервис»
-- (чекбокс не отмечен — долг остаётся) и одна на «Телевидении» (чекбокс
-- отмечен — сносятся вместе с правилом). Даты старше сидовых просрочек
-- аренды (55 месяцев): секция «Просроченные» показывает 50 старейших (asc),
-- карточки целей должны попадать в первую порцию.
    ('77777777-7777-4777-8777-777777777781',
     '11111111-1111-4111-8111-111111111111',
     '46464646-4646-4646-8646-464646464646',
     '55555555-5555-4555-8555-555555555559',
     'payment', (CURRENT_DATE - (61 || ' month')::interval)::date, NULL, 'planned', 'expense',
     'Консьерж-сервис', 50000, 'transfer', 'Консьерж', 'concierge'),
    ('77777777-7777-4777-8777-777777777782',
     '11111111-1111-4111-8111-111111111111',
     '46464646-4646-4646-8646-464646464646',
     '55555555-5555-4555-8555-555555555559',
     'payment', (CURRENT_DATE - (62 || ' month')::interval)::date, NULL, 'planned', 'expense',
     'Консьерж-сервис', 50000, 'transfer', 'Консьерж', 'concierge'),
    ('77777777-7777-4777-8777-777777777783',
     '11111111-1111-4111-8111-111111111111',
     '46464646-4646-4646-8646-464646464646',
     '55555555-5555-4555-8555-55555555555a',
     'payment', (CURRENT_DATE - (63 || ' month')::interval)::date, NULL, 'planned', 'expense',
     'Телевидение', 70000, 'transfer', 'Телевидение', 'tv'),
-- Оплаченный факт «Телевидения» (#467): после удаления правила с чекбоксом
-- оплаченная операция остаётся с пометкой «платёж удалён» (payment_id → NULL).
    ('77777777-7777-4777-8777-777777777784',
     '11111111-1111-4111-8111-111111111111',
     '46464646-4646-4646-8646-464646464646',
     '55555555-5555-4555-8555-55555555555a',
     'payment', (CURRENT_DATE - (64 || ' month')::interval)::date,
     (CURRENT_DATE - (64 || ' month')::interval)::date, 'paid', 'expense',
     'Телевидение', 70000, 'transfer', 'Телевидение', 'tv')
ON CONFLICT (id) DO UPDATE
SET date = EXCLUDED.date,
    paid_date = EXCLUDED.paid_date,
    status = EXCLUDED.status;

-- История платежей (#466): 55 оплаченных вхождений «Интернета» — пара
-- «сегодня/вчера» для групп и 53 помесячно назад (три порции по 50 →
-- скролл-догрузка). Оплаченные даты раньше `since` (он в будущем) — тот же
-- сидовый приём, что у просрочки аренды: тик paid-строки не трогает.
INSERT INTO operations (id, owner_id, property_id, payment_id, origin, date,
                        paid_date, status, type, title, amount_kopecks,
                        payment_form, category_label, category_slug)
VALUES
    ('77777777-7777-4777-8777-000000000001',
     '11111111-1111-4111-8111-111111111111',
     '33333333-3333-4333-8333-333333333333',
     '55555555-5555-4555-8555-555555555556',
     'payment', CURRENT_DATE, CURRENT_DATE, 'paid', 'expense',
     'Интернет', 100000, 'transfer', 'Интернет', 'internet'),
    ('77777777-7777-4777-8777-000000000002',
     '11111111-1111-4111-8111-111111111111',
     '33333333-3333-4333-8333-333333333333',
     '55555555-5555-4555-8555-555555555556',
     'payment', CURRENT_DATE - 1, CURRENT_DATE - 1, 'paid', 'expense',
     'Интернет', 100000, 'transfer', 'Интернет', 'internet')
ON CONFLICT (id) DO UPDATE
SET date = EXCLUDED.date,
    paid_date = EXCLUDED.paid_date,
    status = EXCLUDED.status;

INSERT INTO operations (id, owner_id, property_id, payment_id, origin, date,
                        paid_date, status, type, title, amount_kopecks,
                        payment_form, category_label, category_slug)
SELECT
    ('77777777-7777-4777-8777-' || lpad((77777700 + g)::text, 12, '0'))::uuid,
    '11111111-1111-4111-8111-111111111111',
    '33333333-3333-4333-8333-333333333333',
    '55555555-5555-4555-8555-555555555556',
    'payment',
    (CURRENT_DATE - (g || ' month')::interval)::date,
    (CURRENT_DATE - (g || ' month')::interval)::date,
    'paid', 'expense', 'Интернет', 100000, 'transfer', 'Интернет', 'internet'
FROM generate_series(1, 53) AS g
ON CONFLICT (id) DO NOTHING;

-- Полный список просроченных (#466): 55 просроченных вхождений «Аренды
-- студии» (даты — месяцы назад; две порции по 50 → скролл-догрузка).
INSERT INTO operations (id, owner_id, property_id, payment_id, origin, date,
                        paid_date, status, type, title, amount_kopecks,
                        payment_form, category_label, category_slug)
SELECT
    ('77777777-7777-4777-8777-' || lpad((77777800 + g)::text, 12, '0'))::uuid,
    '11111111-1111-4111-8111-111111111111',
    '46464646-4646-4646-8646-464646464646',
    '55555555-5555-4555-8555-555555555558',
    'payment',
    (CURRENT_DATE - (g || ' month')::interval)::date,
    NULL,
    'planned', 'expense', 'Аренда студии', 3000000, 'transfer', 'Аренда студии', 'rent'
FROM generate_series(1, 55) AS g
ON CONFLICT (id) DO NOTHING;

-- Подписка pro (лимит 5) у сид-владельца (#483): без неё лимит active
-- property = 0 (подписки нет), вход «Добавить объект» превращается в
-- «Сменить тариф», а POST /properties отвечает «лимит исчерпан» — ни вход
-- в визард, ни полный флоу создания в спеках не воспроизвести. Переехало
-- из live-overlay.sql: подписка меняет не количества сида, а доступность
-- входа, и нужна и обычным прогонам e2e, и живым приёмкам. Тариф ищется
-- по имени: id тарифов генерирует бэкенд при сида. Приёмки, которым нужен
-- исчерпанный лимит, ставят своё поверх.
INSERT INTO user_subscriptions (id, user_id, tariff_id, source, status,
                                current_period)
VALUES ('99999999-9999-4999-8999-999999999901',
        '11111111-1111-4111-8111-111111111111',
        (SELECT id FROM tariffs WHERE name = 'pro'),
        'service', 'active', 'month')
ON CONFLICT (id) DO UPDATE
SET tariff_id = EXCLUDED.tariff_id,
    status = EXCLUDED.status,
    current_period = EXCLUDED.current_period;
