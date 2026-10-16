SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Завершение аренды удаляет Платёж арендной платы (ревизия ADR 0053
-- 2026-10-06, тикет #1161, решение владельца): вместо остановки — семантика
-- готового DeletePayment(keep_overdue=true). Просрочка переживает платёж
-- долгом объекта (FK operations.payment_id ON DELETE SET NULL + origin
-- 'payment' — 000115), оплаченные операции остаются в истории объекта, а
-- условия платежа — сумма, день оплаты, автоплатёж — архивируются на аренде
-- («Архив условий»): после смерти платежа это единственный источник чтения
-- «Арендной платы», «Дня оплаты» и «Срока аренды» завершённой. Ссылка
-- payment_id — nullable (RESTRICT и UNIQUE сохраняются: пока ссылка жива,
-- платёж не удалить мимо аренды; снимает её только сама аренда — Завершение
-- или Удаление). Уже завершённые аренды бекфиллятся здесь же: остановленные
-- платежи сносятся так же, поведение единое.

-- 1. Схема: ссылка становится необязательной, появляются колонки архива.
ALTER TABLE rentals ALTER COLUMN payment_id DROP NOT NULL;
ALTER TABLE rentals
    ADD COLUMN rent_amount_kopecks BIGINT CHECK (rent_amount_kopecks >= 0),
    ADD COLUMN rent_payment_day INT CHECK (rent_payment_day BETWEEN 1 AND 31),
    ADD COLUMN rent_auto_pay BOOLEAN;

-- 2. Бекфилл архива: снимок условий с ещё живых платежей завершённых аренд.
-- День оплаты читается из monthly-recurrence платежа: 1..30 — свой день
-- (у арендного платежа он ровно один), «последний день месяца» — 31 (одно
-- поведение, решение №5).
UPDATE rentals r
SET rent_amount_kopecks = p.amount_kopecks,
    rent_payment_day = CASE
        WHEN coalesce((p.recurrence->>'lastDay')::boolean, false) THEN 31
        ELSE (p.recurrence->'daysOfMonth'->>0)::int
    END,
    rent_auto_pay = p.auto_pay
FROM payments p
WHERE r.completed_date IS NOT NULL
  AND r.payment_id = p.id;

-- 3. Снос остановленных платежей, шаг keep_overdue: planned с «сегодня»
-- собственника (его TZ, ADR 0048) и позже удаляются; просрочка остаётся
-- долгом — при удалении платежа FK обнулит operations.payment_id, origin
-- сохранится.
DELETE FROM operations o
USING rentals r, users u
WHERE r.completed_date IS NOT NULL
  AND r.payment_id IS NOT NULL
  AND o.payment_id = r.payment_id
  AND o.status = 'planned'
  AND o.date >= (now() AT TIME ZONE u.timezone)::date
  AND u.id = r.owner_id;

-- 4. Снять ссылку (RESTRICT отпускает платёж), затем удалить сами платежи:
-- идентификаторы перекочёвывают во временную таблицу до обнуления ссылок.
CREATE TEMP TABLE tmp_completed_rental_payments ON COMMIT DROP AS
SELECT DISTINCT payment_id FROM rentals
WHERE completed_date IS NOT NULL AND payment_id IS NOT NULL;

UPDATE rentals SET payment_id = NULL
WHERE completed_date IS NOT NULL AND payment_id IS NOT NULL;

DELETE FROM payments WHERE id IN (SELECT payment_id FROM tmp_completed_rental_payments);

-- 5. Жизненный цикл пары аренда↔платёж — дюрально: незавершённая всегда со
-- ссылкой, завершённая — всегда без (платёж удалён); архив присутствует
-- ровно у завершённой.
ALTER TABLE rentals
    ADD CONSTRAINT rentals_payment_link_lifecycle_check
        CHECK ((completed_date IS NULL) = (payment_id IS NOT NULL)),
    ADD CONSTRAINT rentals_archive_lifecycle_check
        CHECK ((completed_date IS NOT NULL) = (rent_amount_kopecks IS NOT NULL
               AND rent_payment_day IS NOT NULL AND rent_auto_pay IS NOT NULL));
