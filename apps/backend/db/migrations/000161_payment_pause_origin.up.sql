-- Происхождение интервала паузы (тикет #1342): системная автопауза ставится
-- архивацией объекта (ArchiveProperty) всем живым правилам — мимо
-- rent-managed гейта (реакция на объект, не правка условий аренды,
-- ADR 0053 §3), а возврат объекта из архива снимает ТОЛЬКО auto_archive-
-- паузы; ручные остаются. Существующим строкам DEFAULT даёт 'manual' —
-- до тикета все паузы ставил только пользователь.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

ALTER TABLE payment_pauses
    ADD COLUMN origin TEXT NOT NULL DEFAULT 'manual'
    CONSTRAINT payment_pauses_origin_check CHECK (origin IN ('manual', 'auto_archive'));

-- Отметка происхождения в журнале изменений: строку системной паузы/снятия
-- пишет тот же архивный мост, словарь общий с payment_pauses; NULL = ручное
-- действие (все строки до тикета, включая арендный конвейер).
ALTER TABLE payment_change_log
    ADD COLUMN origin TEXT
    CONSTRAINT payment_change_log_origin_check CHECK (origin IN ('auto_archive'));
