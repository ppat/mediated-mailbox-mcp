-- +goose Up
-- The UI's read of an account's progress, which its system endpoint and jobs cards show, arrives with
-- the statement in db/accountstate that makes it (ADR-0084, ADR-0091). The grant is the columns that
-- statement reads, all of them non-secret. It never covers credential, so the UI cannot read a sealed
-- credential even though it seals them.
GRANT SELECT (
    account_id,
    backfill_pass1_complete,
    backfill_pass2_complete,
    sync_cursor_at,
    last_auth_at,
    last_auth_outcome
) ON account_state TO mediated_mailbox_ui;
