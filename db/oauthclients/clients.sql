-- name: OAuthClients :many
-- Every stored OAuth client, one for each provider that authenticates through one (ADR-0080,
-- ADR-0083). The table belongs to no account.
SELECT
    provider,
    client_id,
    client_secret AS sealed_client_secret
FROM oauth_clients
ORDER BY provider;
