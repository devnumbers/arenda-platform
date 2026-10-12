SET lock_timeout = '1s';
SET statement_timeout = '5s';

ALTER TABLE action_journal
    DROP COLUMN details;
