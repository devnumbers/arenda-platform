SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- DB-level date-order invariant for leases, closing the gap found by the
-- schema audit (#322 research; decision #324). The domain allows equality —
-- a one-day lease (ErrEndDateBeforeStart = "end date must be on or after
-- start date", internal/leases/domain/lease.go) — so the strict variant from
-- the research was rejected. end_date NULL = open-ended lease.
ALTER TABLE leases ADD CONSTRAINT chk_leases_end_not_before_start
    CHECK (end_date IS NULL OR end_date >= start_date);
