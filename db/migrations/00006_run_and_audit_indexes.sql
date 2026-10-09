-- +goose Up
-- The indexes the reads that repeat need (ADR-0016). Every runtime role reads under row-level
-- security, and each index leads with the account and compares by leakproof equality and range, so the
-- account's policy and the index serve together.
--
-- A job kind's runs by pass, newest first, for every tick's latest run, the jobs screen's latest run
-- per job kind and pass, and the job mechanism's due decisions. It ends with run_id because the
-- statements break ties on it, in either direction.
CREATE INDEX ON job_runs (account_id, workload, pass, started_at DESC, run_id);
-- The runs still running, which the event stream follows.
CREATE INDEX ON job_runs (account_id) WHERE state = 'running';
-- The runs finished since a time, which the event stream follows too.
CREATE INDEX ON job_runs (account_id, finished_at);
-- The runs dataset, newest first.
CREATE INDEX ON job_runs (account_id, started_at DESC);
-- A message's audit rows, newest first.
CREATE INDEX ON audit_log (account_id, message_id, ts DESC);
