ALTER TABLE operations
    DROP CONSTRAINT IF EXISTS chk_operations_category;

ALTER TABLE operations
    ADD CONSTRAINT chk_operations_category
        CHECK (category IN ('rent', 'other_income', 'utilities', 'repair', 'tax', 'other_expense'));

ALTER TABLE recurring_operations
    DROP CONSTRAINT IF EXISTS chk_recurring_operations_category;

ALTER TABLE recurring_operations
    ADD CONSTRAINT chk_recurring_operations_category
        CHECK (category IN ('rent', 'other_income', 'utilities', 'repair', 'tax', 'other_expense'));
