-- Reminders E2E DB verification
-- Usage: psql -h localhost -p 5433 -U arenda -d arenda -v owner_id='...' -v property_id='...' -v operation_id='...' -v lease_id='...' -f reminders-e2e-verify.sql

\echo '=== Reminders E2E DB Verification ==='

\echo '\n--- Total reminders by status ---'
SELECT status, COUNT(*) AS count
FROM reminders
GROUP BY status
ORDER BY status;

\echo '\n--- Sent SMS reminders count ---'
SELECT COUNT(*) AS sent_sms_count FROM sent_sms_reminders;

\echo '\n--- Active (pending/sending) reminders for test owner ---'
SELECT id, target_type, event_type, status, scheduled_at::date AS scheduled_date
FROM reminders
WHERE owner_id = :owner_id::uuid
  AND status IN ('pending', 'sending')
ORDER BY scheduled_at;

\echo '\n--- Active reminders for test property ---'
SELECT id, target_type, event_type, status, scheduled_at::date AS scheduled_date
FROM reminders
WHERE property_id = :property_id::uuid
  AND status IN ('pending', 'sending')
ORDER BY scheduled_at;

\echo '\n--- Active reminders for test operation ---'
SELECT id, target_type, event_type, status, scheduled_at::date AS scheduled_date
FROM reminders
WHERE operation_id = :operation_id::uuid
  AND status IN ('pending', 'sending')
ORDER BY scheduled_at;

\echo '\n--- Active reminders for test lease ---'
SELECT id, target_type, event_type, status, scheduled_at::date AS scheduled_date
FROM reminders
WHERE lease_id = :lease_id::uuid
  AND status IN ('pending', 'sending')
ORDER BY scheduled_at;

\echo '\n--- Reminders with failed status ---'
SELECT id, owner_id, target_type, event_type, status, failed_attempts, scheduled_at::date AS scheduled_date
FROM reminders
WHERE status = 'failed'
ORDER BY scheduled_at;

\echo '\n--- Recent sent reminders for test owner ---'
SELECT r.id, r.target_type, r.event_type, r.status, r.scheduled_at::date AS scheduled_date, r.sent_at
FROM reminders r
WHERE r.owner_id = :owner_id::uuid
  AND r.status = 'sent'
ORDER BY r.sent_at DESC
LIMIT 10;

\echo '\n--- Recent sent_sms_reminders rows ---'
SELECT s.id, s.reminder_id, s.phone, s.sent_at, s.provider_response
FROM sent_sms_reminders s
JOIN reminders r ON r.id = s.reminder_id
WHERE r.owner_id = :owner_id::uuid
ORDER BY s.sent_at DESC
LIMIT 10;
