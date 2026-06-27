-- Revert the test admin back to a regular owner.
UPDATE users
SET role = 'owner'
WHERE phone = '+79150380663';
