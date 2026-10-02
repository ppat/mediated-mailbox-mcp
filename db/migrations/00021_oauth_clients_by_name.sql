-- +goose Up
-- An installation holds any number of OAuth clients for a provider that authenticates through one,
-- each named by the person running the installation, and each account names the client it connects
-- through (ADR-0106, ADR-0016).
--
-- The client already stored takes its provider's value as its name. A client's secret is sealed to
-- the row its name keys (ADR-0088), so the stored secret stays bound to its row and is not sealed
-- again. Every account of that provider names it.

ALTER TABLE oauth_clients ADD COLUMN name text;
UPDATE oauth_clients SET name = provider;
ALTER TABLE oauth_clients ALTER COLUMN name SET NOT NULL;
ALTER TABLE oauth_clients DROP CONSTRAINT oauth_clients_pkey;
ALTER TABLE oauth_clients ADD PRIMARY KEY (name);
-- What an account's reference names, so the reference carries the provider.
ALTER TABLE oauth_clients ADD UNIQUE (name, provider);
-- One client is never stored twice under two names.
ALTER TABLE oauth_clients ADD UNIQUE (provider, client_id);

-- The client the account connects through, NULL for a provider without one. The reference carries
-- the provider, so an account cannot name another provider's client, and a client an account
-- connects through cannot be removed. The schema does not know which providers use a client, so an
-- account of such a provider that names none is refused service by the account snapshot, which the
-- deployable that calls the provider tells which providers authenticate through a client, never by
-- the schema (ADR-0106).
ALTER TABLE accounts ADD COLUMN oauth_client text;
UPDATE accounts SET oauth_client = provider
WHERE provider IN (SELECT name FROM oauth_clients);
ALTER TABLE accounts ADD FOREIGN KEY (oauth_client, provider) REFERENCES oauth_clients (name, provider);

-- The account snapshot's statements in db/accounts and db/oauthclients read the client each account
-- names and each client's name, under the role of each deployable that calls a provider, whose list
-- admits accountload (ADR-0075, ADR-0090). The UI's role reads accounts in full already, and its
-- grants on oauth_clients arrive with its client setup's statements (ADR-0084).
GRANT SELECT (oauth_client) ON accounts
TO mediated_mailbox_mediate, mediated_mailbox_backfill, mediated_mailbox_sync, mediated_mailbox_organize;
GRANT SELECT (name) ON oauth_clients
TO mediated_mailbox_mediate, mediated_mailbox_backfill, mediated_mailbox_sync, mediated_mailbox_organize;
