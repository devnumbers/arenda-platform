SET lock_timeout = '1s';
SET statement_timeout = '5s';

ALTER TABLE leases DROP CONSTRAINT IF EXISTS chk_leases_end_not_before_start;
