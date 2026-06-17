CREATE INDEX idx_leases_open_past_end ON leases(status, end_date) WHERE end_date IS NOT NULL;
