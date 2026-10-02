SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Зеркальный возврат к правилу 000119: удаление объекта обнуляет ссылку
-- (ON DELETE SET NULL), контакт выживает в книге владельца — решение ADR
-- 0054 §2 в исходной редакции. Схема без данных: контакты, уже снесённые
-- каскадом под действием up-миграции, не восстанавливаются (прецедент
-- 000119.down — down-миграции восстанавливают схему, не данные).
ALTER TABLE contacts
    DROP CONSTRAINT contacts_property_id_fkey,
    ADD CONSTRAINT contacts_property_id_fkey
        FOREIGN KEY (property_id) REFERENCES properties(id) ON DELETE SET NULL;
