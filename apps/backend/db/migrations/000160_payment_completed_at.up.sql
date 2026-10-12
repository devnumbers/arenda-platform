-- Хранимый признак завершённости правила платежа (тикет #1340): completed_at
-- ставит только явная мутация complete (POST .../payments/{paymentId}/complete),
-- ничего не выводит его заново — вычисляемый isCompleted (глоссарий
-- «Завершённый платёж») остаётся read-side флагом, по нему фронт показывает
-- «Полностью оплачен» и кнопку «Завершить» ДО завершения. NULL = правило живо.
-- Завершённое правило не порождает операций (тик и листинги отсекают по
-- IS NULL) и скрыто из лент, списков и подсчётов — те же SQL-гейты; строка
-- жива для прямых чтений (детали, оплаченные операции, журналы).
-- Отдельного индекса нет: IS NULL сужает уже существующие выборки правил
-- (скоуп владельца/видимости), отдельный индекс удвоил бы write-overhead при
-- нуле читателей — как details в 000159.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

ALTER TABLE payments
    ADD COLUMN completed_at TIMESTAMPTZ;

-- Действие `completed` в журнале изменений (тикет #1340): строка завершения
-- с пустым changes — форма паузы/возобновления (ADR 0065 §4).
ALTER TABLE payment_change_log
    DROP CONSTRAINT payment_change_log_action_check;
ALTER TABLE payment_change_log
    ADD CONSTRAINT payment_change_log_action_check
    CHECK (action IN ('updated', 'paused', 'resumed', 'completed'));
