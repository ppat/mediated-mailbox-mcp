-- name: AddAccount :exec
-- Writes a new account's listing row with the client it connects through, in the transaction that
-- writes its state row (ADR-0091, ADR-0106). The primary key refuses an identifier another account
-- holds.
INSERT INTO accounts (account_id, provider, oauth_client)
VALUES (@account_id, @provider, @oauth_client);

-- name: AddState :exec
-- Writes a connected account's state row, its sealed credential, the mailbox the consent confirmed
-- and the lowered target the operator set, or NULL for none (ADR-0080, ADR-0081, ADR-0024).
INSERT INTO account_state (account_id, credential, mailbox, lowered_target_rate)
VALUES (@account_id, @credential, @mailbox, @lowered_target);

-- name: Reauthorize :execrows
-- Replaces a re-authorized account's credential, unconditionally, since the operator's write is the
-- newest (ADR-0089). The mailbox is written only where the row remembers none, so a remembered one
-- never changes (ADR-0080). No row means the account has no state row.
UPDATE account_state
SET
    credential = @credential,
    mailbox = coalesce(mailbox, sqlc.arg(mailbox)::text)
WHERE account_id = @account_id;

-- name: MoveAccount :execrows
-- Moves the account to another client of its provider, in the transaction that writes the credential
-- issued to that client (ADR-0106).
UPDATE accounts
SET oauth_client = @oauth_client
WHERE account_id = @account_id AND provider = @provider;

-- name: AccountSetup :many
-- What account setup and account settings read of the account, its provider and client, whether it
-- has a state row, the mailbox it remembers and its lowered target. It reads no credential (ADR-0084,
-- ADR-0091). No row means no account holds the identifier.
SELECT
    a.provider AS account_provider,
    a.oauth_client,
    s.mailbox,
    s.lowered_target_rate,
    (s.account_id IS NOT NULL)::boolean AS connected
FROM accounts AS a
LEFT JOIN account_state AS s ON a.account_id = s.account_id
WHERE a.account_id = @account_id;

-- name: SetLoweredTarget :execrows
-- Stores the lowered target the operator set, a fraction of the provider's declared ceiling, or NULL
-- for none (ADR-0024). No row means the account has no state row.
UPDATE account_state
SET lowered_target_rate = sqlc.narg(lowered_target)
WHERE account_id = @account_id;
