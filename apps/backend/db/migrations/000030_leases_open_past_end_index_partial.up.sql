DROP INDEX IF EXISTS idx_leases_open_past_end;
CREATE INDEX idx_leases_open_past_end ON leases(end_date)
WHERE status IN ('awaiting_start','active','requires_action') AND end_date IS NOT NULL;
