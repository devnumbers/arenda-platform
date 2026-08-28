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
                      recurrence, since, end_date, auto_pay, payment_form,
                      category_slug, is_favorite)
VALUES
-- Еженедельный доход: одна суббота в неделю (0=Вс…6=Сб).
    ('55555555-5555-4555-8555-555555555561',
     '11111111-1111-4111-8111-111111111111',
     '33333333-3333-4333-8333-333333333333',
     'income', 'Аренда машиноместа', 200000,
     '{"kind": "weekly", "weekdays": [6]}',
     CURRENT_DATE + 3, NULL, FALSE, 'transfer', 'parking', FALSE),
-- Доход с окончанием в будущем: «Ближайший платеж» с датой, признак
-- завершённости ещё не наступил.
    ('55555555-5555-4555-8555-555555555562',
     '11111111-1111-4111-8111-111111111111',
     '33333333-3333-4333-8333-333333333333',
     'income', 'Компенсация ЖКУ', 350000,
     '{"kind": "monthly", "dayOfMonth": 10}',
     CURRENT_DATE + 3, CURRENT_DATE + 90, FALSE, 'cash', 'utilities-compensation', FALSE),
-- Еженедельный расход на два дня недели — проверка метки повторяемости
-- вида «Каждый понедельник и четверг» (recurrenceLabel).
    ('55555555-5555-4555-8555-555555555563',
     '11111111-1111-4111-8111-111111111111',
     '33333333-3333-4333-8333-333333333333',
     'expense', 'Клининг холла', 150000,
     '{"kind": "weekly", "weekdays": [1, 4]}',
     CURRENT_DATE + 3, NULL, FALSE, 'transfer', 'cleaning', FALSE)
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
