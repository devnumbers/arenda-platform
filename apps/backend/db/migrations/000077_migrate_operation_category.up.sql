-- 1. Add nullable category_id
ALTER TABLE operations
    ADD COLUMN IF NOT EXISTS category_id UUID REFERENCES operation_categories(id) ON DELETE RESTRICT;

ALTER TABLE recurring_operations
    ADD COLUMN IF NOT EXISTS category_id UUID REFERENCES operation_categories(id) ON DELETE RESTRICT;

-- 2. Seed default categories for every existing user
INSERT INTO operation_categories (owner_id, type, name, code)
SELECT id, 'income', 'rent', 'rent' FROM users
UNION ALL
SELECT id, 'income', 'other_income', 'other_income' FROM users
UNION ALL
SELECT id, 'expense', 'utilities', 'utilities' FROM users
UNION ALL
SELECT id, 'expense', 'repair', 'repair' FROM users
UNION ALL
SELECT id, 'expense', 'tax', 'tax' FROM users
UNION ALL
SELECT id, 'expense', 'other_expense', 'other_expense' FROM users
UNION ALL
SELECT id, 'expense', 'deposit_return', 'deposit_return' FROM users
ON CONFLICT (owner_id, type, lower(name)) DO NOTHING;

-- 3. Backfill operations.category_id
UPDATE operations
SET category_id = oc.id
FROM operation_categories oc
WHERE operations.owner_id = oc.owner_id
  AND operations.type = oc.type
  AND lower(operations.category) = lower(oc.name);

-- 4. Backfill recurring_operations.category_id
UPDATE recurring_operations
SET category_id = oc.id
FROM operation_categories oc
WHERE recurring_operations.owner_id = oc.owner_id
  AND recurring_operations.type = oc.type
  AND lower(recurring_operations.category) = lower(oc.name);

-- 5. Make category_id NOT NULL
ALTER TABLE operations
    ALTER COLUMN category_id SET NOT NULL;

ALTER TABLE recurring_operations
    ALTER COLUMN category_id SET NOT NULL;

-- 6. Drop CHECK constraints
ALTER TABLE operations
    DROP CONSTRAINT IF EXISTS chk_operations_category;

ALTER TABLE recurring_operations
    DROP CONSTRAINT IF EXISTS chk_recurring_operations_category;

-- 7. Drop the old category column
ALTER TABLE operations
    DROP COLUMN IF EXISTS category;

ALTER TABLE recurring_operations
    DROP COLUMN IF EXISTS category;
