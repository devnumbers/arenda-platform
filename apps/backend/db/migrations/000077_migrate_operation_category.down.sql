-- Reverse schema changes: restore category column and CHECK constraints, drop category_id.
ALTER TABLE operations
    ADD COLUMN IF NOT EXISTS category TEXT;

ALTER TABLE recurring_operations
    ADD COLUMN IF NOT EXISTS category TEXT;

UPDATE operations
SET category = oc.code
FROM operation_categories oc
WHERE operations.category_id = oc.id;

UPDATE recurring_operations
SET category = oc.code
FROM operation_categories oc
WHERE recurring_operations.category_id = oc.id;

ALTER TABLE operations
    ALTER COLUMN category SET NOT NULL;

ALTER TABLE recurring_operations
    ALTER COLUMN category SET NOT NULL;

ALTER TABLE operations
    ADD CONSTRAINT chk_operations_category
        CHECK (category IN ('rent', 'other_income', 'utilities', 'repair', 'tax', 'other_expense', 'deposit_return'));

ALTER TABLE recurring_operations
    ADD CONSTRAINT chk_recurring_operations_category
        CHECK (category IN ('rent', 'other_income', 'utilities', 'repair', 'tax', 'other_expense', 'deposit_return'));

ALTER TABLE operations
    DROP COLUMN IF EXISTS category_id;

ALTER TABLE recurring_operations
    DROP COLUMN IF EXISTS category_id;
