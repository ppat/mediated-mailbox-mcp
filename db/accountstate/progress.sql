-- name: AccountProgress :one
-- An account's progress through its workloads and its last provider authentication, which the system
-- endpoint's operational block and the jobs backfill card read (docs/UI.md sections 8.3 and 8.8). It
-- reads no credential, and the role that runs it holds no grant on one (ADR-0084, ADR-0091).
SELECT
    s.backfill_pass1_complete,
    s.backfill_pass2_complete,
    s.sync_cursor_at,
    s.last_auth_at,
    s.last_auth_outcome
FROM account_state AS s
WHERE s.account_id = @account_id;
