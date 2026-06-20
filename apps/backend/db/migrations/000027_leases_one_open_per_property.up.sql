-- Note: resolve any existing duplicate open leases per property before applying.
CREATE UNIQUE INDEX idx_leases_one_open_per_property
ON leases(property_id)
WHERE status IN ('awaiting_start', 'active', 'requires_action');
