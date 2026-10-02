-- name: Accounts :many
-- Every account's identifier, provider and the OAuth client it connects through, NULL for a provider
-- without one, the one statement left without an account predicate. The roles that list accounts
-- read every row of the table (ADR-0091, ADR-0106).
SELECT
    account_id,
    provider AS account_provider,
    oauth_client
FROM accounts
ORDER BY account_id;
