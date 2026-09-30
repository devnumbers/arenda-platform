SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Индексы чтения по фактической дате оплаты (карта #990, тикет #992):
-- операционные ленты при sort=paid_date сортируют, фильтруют периодом и
-- листают ключсетом по paid_date — зеркальная пара индексов 000115
-- (idx_operations_{property,owner}_date): скоуп (объект объектных лент,
-- владелец глобальной) → paid_date → id-тайбрейк. Полные, не частичные:
-- смешанный скоуп объектных поверхностей несёт плановые строки (paid_date
-- IS NULL) с NULLS LAST, частичный индекс порядок полного списка не дал бы.
-- Обратный проход обслуживает desc платёжной ленты (paid-only, NULL нет);
-- desc с NULLS LAST смешанного объектного скоупа индексным порядком не
-- читается — планировщик добавляет сортировку, индекс остаётся срезом
-- скоупа (объёмы малы, конвенция 000124/000115).
CREATE INDEX idx_operations_property_paid_date_id
    ON operations(property_id, paid_date, id);

CREATE INDEX idx_operations_owner_paid_date_id
    ON operations(owner_id, paid_date, id);
