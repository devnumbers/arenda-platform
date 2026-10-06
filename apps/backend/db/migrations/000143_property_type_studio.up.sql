SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Студия — полноценное значение типа (карта #984, тикет #1003): расширение
-- CHECK-констрейнта properties.type значением 'studio' (expand, существующие
-- строки не трогаются; откат возвращает список из десяти типов 000002).
ALTER TABLE properties DROP CONSTRAINT properties_type_check;
ALTER TABLE properties ADD CONSTRAINT properties_type_check
CHECK (type IN ('apartment','room','apartments','studio','house','commercial','office','warehouse','garage','parking','land'));
