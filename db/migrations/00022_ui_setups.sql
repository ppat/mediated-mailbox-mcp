-- +goose Up
-- The UI's OAuth client setup and account setup (ADR-0084, ADR-0106, ADR-0107, ADR-0080).
--
-- A client remembers the provider's project it belongs to, Google Cloud's project ID for Gmail,
-- which client setup writes and the installation screen links to. It is not secret, and NULL when
-- the setup was given none (ADR-0016).
ALTER TABLE oauth_clients ADD COLUMN project_id text;

-- An account remembers the mailbox the consent that first stored its credential confirmed, and
-- re-authorizing it refuses a grant for any other (ADR-0080).
ALTER TABLE account_state ADD COLUMN mailbox text;

-- No account identifier is exactly ., .. or /, which the mediator's API root cannot address as a path
-- segment (ADR-0087). Account setup refuses them first, with its own wording.
ALTER TABLE accounts ADD CONSTRAINT accounts_account_id_addressable
CHECK (account_id NOT IN ('.', '..', '/'));

-- The UI's client setup lists each client's identity, adds a client, replaces a client's identifier,
-- secret and project ID, and removes a client no account connects through. Its statements sit in
-- db/oauthclients/setup and read no secret. The one part of the UI that opens a client's secret, for a
-- consent's code exchange, reads the sealed secret through db/oauthclients, which only that part's list
-- admits, and the raw SQL analyser refuses a statement the UI's shipped code runs by hand (ADR-0071,
-- ADR-0081). Code written to get around both, such as a statement run through reflection, stays
-- with review. The table has no row-level security, so the grant is the whole
-- database barrier (ADR-0016, ADR-0084, ADR-0106).
GRANT SELECT (name, provider, client_id, client_secret, project_id),
INSERT (name, provider, client_id, client_secret, project_id),
UPDATE (client_id, client_secret, project_id),
DELETE ON oauth_clients TO mediated_mailbox_ui;

-- Account setup writes an account's listing row, with the client it connects through, and moves an
-- account to another client of its provider (ADR-0091, ADR-0106). accounts_account confines both
-- writes to the transaction's account. Its statements sit in db/accounts/setup.
GRANT INSERT (account_id, provider, oauth_client), UPDATE (oauth_client) ON accounts TO mediated_mailbox_ui;

-- Account setup writes the account's state row, its sealed credential, the mailbox it remembers and
-- its lowered target, replaces the credential on a re-authorization, and records the code exchange's
-- authentication attempt through db/accountstate/authentication's latest-wins statement (ADR-0097).
-- Account settings reads the mailbox and the lowered target and writes the target. The grant never
-- covers reading credential, so the UI cannot read back what it sealed (ADR-0081, ADR-0084).
GRANT SELECT (mailbox, lowered_target_rate),
INSERT (account_id, credential, mailbox, lowered_target_rate),
UPDATE (credential, mailbox, lowered_target_rate, last_auth_at, last_auth_outcome)
ON account_state TO mediated_mailbox_ui;
