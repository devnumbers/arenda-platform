-- Live-only overlay for the walkthrough stack (E2E_LIVE=1): правила для
-- состояний, которых намеренно нет в базовом seed.sql — еженедельная
-- периодичность (в т.ч. несколько дней недели), доходные правила и
-- окончание в будущем. Базовый сид не расширяется, чтобы не ломать точные
-- количества, на которые опираются Playwright-спеки.
--
-- Применяется раннером (run-frontend-e2e.sh) только в live-режиме, сразу
-- после базового сида. Идемпотентен (ON CONFLICT DO UPDATE). Все `since` в
-- будущем: почасовой тик живого стека ничего не материализует поверх сида,
-- секция «Просроченные» квартиры остаётся сидовой. Объект — квартира
-- «Квартира на Ленина»; гараж намеренно остаётся пустым (пустые состояния).
-- Слаги категорий — из дефолтного каталога
-- (tools/payment-categories/catalog.json).
INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks,
                      recurrence, since, end_date, auto_pay,
                      category_slug, is_favorite)
VALUES
-- Еженедельный доход: одна суббота в неделю (0=Вс…6=Сб).
    ('55555555-5555-4555-8555-555555555561',
     '11111111-1111-4111-8111-111111111111',
     '33333333-3333-4333-8333-333333333333',
     'income', 'Аренда машиноместа', 200000,
     '{"kind": "weekly", "weekdays": [6]}',
     CURRENT_DATE + 3, NULL, FALSE, 'parking', FALSE),
-- Доход с окончанием в будущем: «Ближайший платеж» с датой, признак
-- завершённости ещё не наступил.
    ('55555555-5555-4555-8555-555555555562',
     '11111111-1111-4111-8111-111111111111',
     '33333333-3333-4333-8333-333333333333',
     'income', 'Компенсация ЖКУ', 350000,
     '{"kind": "monthly", "dayOfMonth": 10}',
     CURRENT_DATE + 3, CURRENT_DATE + 90, FALSE, 'utilities-compensation', FALSE),
-- Еженедельный расход на два дня недели — проверка метки повторяемости
-- вида «Каждый понедельник и четверг» (recurrenceLabel).
    ('55555555-5555-4555-8555-555555555563',
     '11111111-1111-4111-8111-111111111111',
     '33333333-3333-4333-8333-333333333333',
     'expense', 'Клининг холла', 150000,
     '{"kind": "weekly", "weekdays": [1, 4]}',
     CURRENT_DATE + 3, NULL, FALSE, 'cleaning', FALSE)
ON CONFLICT (id) DO UPDATE
SET title = EXCLUDED.title,
    amount_kopecks = EXCLUDED.amount_kopecks,
    recurrence = EXCLUDED.recurrence,
    since = EXCLUDED.since,
    end_date = EXCLUDED.end_date,
    auto_pay = EXCLUDED.auto_pay,
    category_slug = EXCLUDED.category_slug,
    is_favorite = EXCLUDED.is_favorite;

-- История «Интернета»: пара офсетных оплат для подписей строки операции
-- (1332:61665): «Заранее на 2 дня» (paid на 2 дня раньше срока) и
-- «Задержан на 2 дня» (paid на 2 дня позже). Даты не пересекаются с сидом
-- (CURRENT_DATE/−1 и помесячными) — дедуп (payment_id, date) не задет.
INSERT INTO operations (id, owner_id, property_id, payment_id, origin, date,
                        paid_date, status, type, title, amount_kopecks,
                        category_label, category_slug)
VALUES
    ('77777777-7777-4777-8777-000000000790',
     '11111111-1111-4111-8111-111111111111',
     '33333333-3333-4333-8333-333333333333',
     '55555555-5555-4555-8555-555555555556',
     'payment', CURRENT_DATE - 3, CURRENT_DATE - 5, 'paid', 'expense',
     'Интернет', 100000, 'Интернет', 'internet'),
    ('77777777-7777-4777-8777-000000000791',
     '11111111-1111-4111-8111-111111111111',
     '33333333-3333-4333-8333-333333333333',
     '55555555-5555-4555-8555-555555555556',
     'payment', CURRENT_DATE - 6, CURRENT_DATE - 4, 'paid', 'expense',
     'Интернет', 100000, 'Интернет', 'internet'),
-- Доходная оплата для зелёного плюса (State=Plus): «Аренда машиноместа»,
-- суббота, оплачена в день срока.
    ('77777777-7777-4777-8777-000000000792',
     '11111111-1111-4111-8111-111111111111',
     '33333333-3333-4333-8333-333333333333',
     '55555555-5555-4555-8555-555555555561',
     'payment', CURRENT_DATE - 7, CURRENT_DATE - 7, 'paid', 'income',
     'Аренда машиноместа', 200000, 'Аренда машиноместа', 'parking')
ON CONFLICT (id) DO UPDATE
SET date = EXCLUDED.date,
    paid_date = EXCLUDED.paid_date,
    status = EXCLUDED.status;

