-- Restore English display names of the seven seeded default operation
-- categories (four current plus three legacy codes). The NOT EXISTS guard
-- skips rows whose English name would collide with an existing category of
-- the same owner and type.
UPDATE operation_categories
SET name = code
WHERE code IN ('rent', 'utilities', 'repair', 'tax', 'other_income', 'other_expense', 'deposit_return')
  AND NOT EXISTS (
      SELECT 1 FROM operation_categories oc2
      WHERE oc2.owner_id = operation_categories.owner_id
        AND oc2.type = operation_categories.type
        AND lower(oc2.name) = lower(operation_categories.code)
        AND oc2.id != operation_categories.id
  );
