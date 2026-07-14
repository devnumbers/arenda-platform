-- Localize display names of the seven seeded default operation categories.
-- Four of them (rent, utilities, repair, tax) are still seeded for new
-- users; the other three (other_income, other_expense, deposit_return) are
-- legacy codes that exist only for users registered before they were
-- removed from seeding (migration 000077).
-- The machine key (code) stays English; only the display name changes.
-- Only seeded system categories are matched (user-created categories have
-- code IS NULL). Operations reference categories by id, so nothing else
-- is affected.
-- The NOT EXISTS guard skips the rename for an owner who already has a
-- category with the same (type, lower(name)), avoiding a violation of
-- idx_operation_categories_owner_type_lower_name.
UPDATE operation_categories
SET name = 'Аренда'
WHERE code = 'rent'
  AND NOT EXISTS (
      SELECT 1 FROM operation_categories oc2
      WHERE oc2.owner_id = operation_categories.owner_id
        AND oc2.type = operation_categories.type
        AND lower(oc2.name) = lower('Аренда')
        AND oc2.id != operation_categories.id
  );

UPDATE operation_categories
SET name = 'Коммунальные услуги'
WHERE code = 'utilities'
  AND NOT EXISTS (
      SELECT 1 FROM operation_categories oc2
      WHERE oc2.owner_id = operation_categories.owner_id
        AND oc2.type = operation_categories.type
        AND lower(oc2.name) = lower('Коммунальные услуги')
        AND oc2.id != operation_categories.id
  );

UPDATE operation_categories
SET name = 'Ремонт'
WHERE code = 'repair'
  AND NOT EXISTS (
      SELECT 1 FROM operation_categories oc2
      WHERE oc2.owner_id = operation_categories.owner_id
        AND oc2.type = operation_categories.type
        AND lower(oc2.name) = lower('Ремонт')
        AND oc2.id != operation_categories.id
  );

UPDATE operation_categories
SET name = 'Налог'
WHERE code = 'tax'
  AND NOT EXISTS (
      SELECT 1 FROM operation_categories oc2
      WHERE oc2.owner_id = operation_categories.owner_id
        AND oc2.type = operation_categories.type
        AND lower(oc2.name) = lower('Налог')
        AND oc2.id != operation_categories.id
  );

-- Legacy codes: present only for users registered before these categories
-- were removed from seeding.
UPDATE operation_categories
SET name = 'Прочий доход'
WHERE code = 'other_income'
  AND NOT EXISTS (
      SELECT 1 FROM operation_categories oc2
      WHERE oc2.owner_id = operation_categories.owner_id
        AND oc2.type = operation_categories.type
        AND lower(oc2.name) = lower('Прочий доход')
        AND oc2.id != operation_categories.id
  );

UPDATE operation_categories
SET name = 'Прочий расход'
WHERE code = 'other_expense'
  AND NOT EXISTS (
      SELECT 1 FROM operation_categories oc2
      WHERE oc2.owner_id = operation_categories.owner_id
        AND oc2.type = operation_categories.type
        AND lower(oc2.name) = lower('Прочий расход')
        AND oc2.id != operation_categories.id
  );

UPDATE operation_categories
SET name = 'Возврат депозита'
WHERE code = 'deposit_return'
  AND NOT EXISTS (
      SELECT 1 FROM operation_categories oc2
      WHERE oc2.owner_id = operation_categories.owner_id
        AND oc2.type = operation_categories.type
        AND lower(oc2.name) = lower('Возврат депозита')
        AND oc2.id != operation_categories.id
  );
