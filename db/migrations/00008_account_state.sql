-- +goose Up
-- The accounts table keeps what every listing needs, each account's identifier and provider, and
-- everything else an account carries moves to account_state, one row per account under the
-- per-account policy (ADR-0091). The state gains the sealed credential (ADR-0080, ADR-0081, ADR-0088)
-- and the lower target rate an operator may set (ADR-0024). An account with no state row is not
-- connected.

CREATE TABLE account_state (
    account_id text PRIMARY KEY REFERENCES accounts,
    -- The account's provider credential, sealed to the current public key and bound to this row
    -- (ADR-0081, ADR-0088). A deployable replaces it only if it still holds the bytes the
    -- deployable last read or wrote (ADR-0089).
    credential bytea,
    -- A lower target the operator set, NULL for none (ADR-0024).
    lowered_target_rate real,
    backfill_pass1_complete boolean NOT NULL DEFAULT false,
    backfill_pass2_complete boolean NOT NULL DEFAULT false,
    sync_cursor text,
    -- When sync_cursor was last written, by delta sync.
    sync_cursor_at timestamptz,
    -- The last provider authentication attempt and its outcome, exposed by ADR-0034.
    last_auth_at timestamptz,
    last_auth_outcome text
);

-- An account's state as the accounts table held it before this migration.
INSERT INTO account_state (
    account_id,
    backfill_pass1_complete,
    backfill_pass2_complete,
    sync_cursor,
    sync_cursor_at,
    last_auth_at,
    last_auth_outcome
)
SELECT
    account_id,
    backfill_pass1_complete,
    backfill_pass2_complete,
    sync_cursor,
    sync_cursor_at,
    last_auth_at,
    last_auth_outcome
FROM accounts;

ALTER TABLE accounts
DROP COLUMN backfill_pass1_complete,
DROP COLUMN backfill_pass2_complete,
DROP COLUMN sync_cursor,
DROP COLUMN sync_cursor_at,
DROP COLUMN last_auth_at,
DROP COLUMN last_auth_outcome;

ALTER TABLE account_state ENABLE ROW LEVEL SECURITY;
CREATE POLICY account_state_account ON account_state
USING (account_id = current_setting('app.account'));

-- The roles that list accounts read all of its rows, one of ADR-0016's exceptions, so a listing
-- needs no account set (ADR-0091). They are the UI's, for its account selector, and the mediator's,
-- backfill's, delta sync's and the reorg workload's, for their account snapshots. Any other role that
-- reads accounts sees only its transaction's account. accounts_account still confines every write
-- to the transaction's account, since a permissive policy for SELECT alone takes no part in an
-- insert, an update's new row or a delete. The UI's read of the moved state columns arrives with the
-- statement that needs it and never covers credential (ADR-0084).
CREATE POLICY accounts_listing ON accounts FOR SELECT
TO mediated_mailbox_ui,
mediated_mailbox_mediate,
mediated_mailbox_backfill,
mediated_mailbox_sync,
mediated_mailbox_organize
USING (true);

-- An installation's OAuth client, for a provider that authenticates through one (ADR-0080, ADR-0083).
-- It belongs to no account and carries no account column, so grants alone decide who reaches it
-- (ADR-0016). A provider that authenticates otherwise has no row, and no account refers to one.
CREATE TABLE oauth_clients (
    provider text PRIMARY KEY,
    client_id text NOT NULL,
    -- The client's secret, sealed to the current public key and bound to this row (ADR-0081, ADR-0088).
    client_secret bytea NOT NULL
);

-- The account snapshot's statements in db/accounts, db/accountstate and db/oauthclients run under the
-- role of each deployable that calls a provider, whose list admits accountload (ADR-0075, ADR-0090).
-- Each lists the accounts, reads an account's sealed credential, writes a rotated credential back by
-- compare-and-set on the bytes it last knew (ADR-0082, ADR-0089), and reads every OAuth client.
GRANT SELECT (account_id, provider) ON accounts
TO mediated_mailbox_mediate, mediated_mailbox_backfill, mediated_mailbox_sync, mediated_mailbox_organize;
GRANT SELECT (account_id, credential), UPDATE (credential) ON account_state
TO mediated_mailbox_mediate, mediated_mailbox_backfill, mediated_mailbox_sync, mediated_mailbox_organize;
GRANT SELECT (provider, client_id, client_secret) ON oauth_clients
TO mediated_mailbox_mediate, mediated_mailbox_backfill, mediated_mailbox_sync, mediated_mailbox_organize;
