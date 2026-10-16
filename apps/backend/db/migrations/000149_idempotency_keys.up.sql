SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Идемпотентные ключи creations (тикет Т3 карты #1112, ресерч #1114):
-- последняя линия защиты POST-созданий от дублей — повтор с тем же ключом
-- возвращает сохранённый результат первой попытки (паттерн Stripe
-- Idempotent Requests; HTTP-стандарта на заголовок нет — draft expired,
-- делаем по де-факто конвенции). Ключ скоупится владельцем (PK
-- (owner_id, key)); request_hash ловит «тот же ключ с другим телом»;
-- NULL status_code = бронь в полёте (гонка двух параллельных запросов
-- с одним ключом: второй получает 409, а не второй 201). Хранилище
-- TTL-ное (24 часа) — чистка случаем в мидлвари, без отдельной очереди.
CREATE TABLE idempotency_keys (
    owner_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key          TEXT        NOT NULL CHECK (char_length(key) BETWEEN 1 AND 255),
    endpoint     TEXT        NOT NULL,
    request_hash TEXT        NOT NULL,
    content_type TEXT,
    status_code  INTEGER,
    response     JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (owner_id, key)
);

CREATE INDEX idx_idempotency_keys_created_at
    ON idempotency_keys (created_at);
