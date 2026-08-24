-- Drops the leases context data and schema entirely (issue #438, spec #434):
-- reminders with their send-audit tables first (FK), then operations,
-- recurring operations and categories, then leases and tenant contacts.
-- Order follows the foreign keys; the down restores the schema without data.
-- The notification_event_type / notification_target_type enum values stay in
-- PostgreSQL as dead values (precedent #277); audit history is not touched.
DROP TABLE sent_push_reminders;
DROP TABLE sent_email_reminders;
DROP TABLE reminders;
DROP TABLE operations;
DROP TABLE recurring_operations;
DROP TABLE operation_categories;
DROP TABLE leases;
DROP TABLE tenant_contacts;
