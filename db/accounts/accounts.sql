-- name: Accounts :many
-- Every account's identifier and provider, the one statement left without an account predicate. The
-- roles that list accounts read every row of the table (ADR-0091).
SELECT
    account_id,
    provider AS account_provider
FROM accounts
ORDER BY account_id;
