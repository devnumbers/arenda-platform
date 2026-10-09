SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Лог изменений платежа (ADR 0065, тикет #1188): append-only история правок
-- правила — одна строка на действие пользователя, диф старое→новое по словарю
-- полей мутационного аудита; пауза/возобновление — строки с пустым changes и
-- своим action; создание и удаление строк не пишут (удаление каскадит журнал
-- за правилом, надгробие остаётся в «Истории действий», ADR 0061).

-- owner_id/property_id — денормализация скоупа чтения (ADR 0028), как у
-- payments/operations; actor_id — пишут только пользователи. changes —
-- типизированные значения [{field, old, new}]: копейки числом, регулярность
-- каноническим JSON, даты «YYYY-MM-DD»|null, категория — ссылка со снапшотом
-- лейбла (канон category_label операций, ADR 0049 §1). Keyset-индекс чтения —
-- обратная хронология по платежу.
CREATE TABLE payment_change_log (
    id UUID PRIMARY KEY,
    payment_id UUID NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    actor_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action TEXT NOT NULL CHECK (action IN ('updated', 'paused', 'resumed')),
    changes JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payment_change_log_payment
    ON payment_change_log (payment_id, created_at DESC, id DESC);

-- Маркер ручного названия (ADR 0065 §3): durable-состояние «название
-- когда-либо задано вручную» — взводится правкой с title, не сбрасывается,
-- созданием не взводится. Читателей сегодня нет; закрепляет правило будущих
-- авто-правок названия (мимо дифа, пока маркер не взведён).
ALTER TABLE payments ADD COLUMN title_is_manual BOOLEAN NOT NULL DEFAULT false;
