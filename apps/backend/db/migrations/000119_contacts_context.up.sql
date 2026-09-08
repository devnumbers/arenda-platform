SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Contacts context schema (ADR 0054, ticket #506): книга контактов владельца —
-- карточки людей для объектов (сантехник, УК, консьерж), не пользователей
-- сервиса. Идентификаторы — UUIDv7 из приложения (ADR 0019); owner_id — книга
-- владельца (ADR 0028 scope), property_id — опциональная привязка к объекту:
-- удаление объекта в обоих режимах ADR 0025 обнуляет ссылку (ON DELETE SET
-- NULL), контакт выживает в книге владельца — осознанный разворот ADR 0026.
CREATE TABLE contacts (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    property_id UUID REFERENCES properties(id) ON DELETE SET NULL,
    first_name TEXT NOT NULL,
    last_name TEXT,
    patronymic TEXT,
    role TEXT,
    phone TEXT,
    email TEXT,
    messenger_name TEXT,
    messenger_username TEXT,
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_contacts_updated_at
    BEFORE UPDATE ON contacts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_contacts_owner ON contacts(owner_id);
CREATE INDEX idx_contacts_property ON contacts(property_id);

-- Снос поверхности ADR 0026 целиком (ADR 0054, решение 4): таблица, данные
-- не переносятся — решение владельца, книга начинается пустой. Down
-- восстанавливает схему без данных (прецедент ADR 0046). Домен/сервис/
-- HTTP-поверхность старой вертикали в properties сносятся соседними
-- изменениями и тикетом #507.
DROP TABLE property_contacts;
