-- name: SetBackfillFirstComplete :exec
-- Records that backfill's first pass has ended for the account, in the transaction that finishes the
-- run which ended it (ADR-0017, ADR-0022).
UPDATE account_state
SET backfill_pass1_complete = true
WHERE account_id = @account_id;
