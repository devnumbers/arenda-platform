-- Откат 000143: список типов до расширения (000002). Миграция падает, если
-- в таблице остались строки со студией — сначала их удалить или сменить тип.
ALTER TABLE properties DROP CONSTRAINT properties_type_check;
ALTER TABLE properties ADD CONSTRAINT properties_type_check
CHECK (type IN ('apartment','room','apartments','house','commercial','office','warehouse','garage','parking','land'));
