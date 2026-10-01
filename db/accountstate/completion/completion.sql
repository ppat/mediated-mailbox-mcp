-- name: SetBackfillFirstComplete :exec
-- Records that backfill's first pass has ended for the account, in the transaction that finishes the
-- run which ended it (ADR-0017, ADR-0022).
UPDATE account_state
SET backfill_pass1_complete = true
WHERE account_id = @account_id;

-- name: SetBackfillSecondComplete :exec
-- Records that backfill's second pass has ended for the account, in the transaction that finishes the
-- run which ended it (ADR-0017, ADR-0022).
UPDATE account_state
SET backfill_pass2_complete = true
WHERE account_id = @account_id;

-- name: ReopenBackfillFirst :exec
-- Records that backfill's first pass is due again for the account, in the transaction that starts the
-- run which runs it again (ADR-0096).
UPDATE account_state
SET backfill_pass1_complete = false
WHERE account_id = @account_id;

-- name: ReopenBackfillSecond :exec
-- Records that backfill's second pass is due again for the account and marks it to start over from the
-- first message waiting for a scan, in the transaction a backfill run makes before its first pass to
-- return the verdicts another scanner made to pending (ADR-0096).
UPDATE account_state
SET backfill_pass2_complete = false, backfill_pass2_restart = true
WHERE account_id = @account_id;

-- name: SecondRestart :one
-- Whether backfill's second pass is marked to start over from the first message waiting for a scan
-- (ADR-0096).
SELECT a.backfill_pass2_restart
FROM account_state AS a
WHERE a.account_id = @account_id;

-- name: ClearSecondRestart :exec
-- Clears the mark that starts backfill's second pass over, in the transaction that records the start of
-- the second pass run that starts, which reads it first (ADR-0096).
UPDATE account_state
SET backfill_pass2_restart = false
WHERE account_id = @account_id;
