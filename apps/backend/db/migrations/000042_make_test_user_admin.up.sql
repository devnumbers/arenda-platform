-- Ensure the test phone number is an admin user.
-- If the user already exists, update the role; otherwise create a seed admin.
INSERT INTO users (id, phone, role, name, created_at, updated_at)
VALUES (gen_random_uuid(), '+79150380663', 'admin', 'Admin', NOW(), NOW())
ON CONFLICT (phone) DO UPDATE SET role = 'admin';
