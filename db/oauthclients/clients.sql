-- name: OAuthClients :many
-- Every stored OAuth client, any number for each provider that authenticates through one, keyed on
-- the name the person running the installation gave it (ADR-0080, ADR-0106). The table belongs to no
-- account.
SELECT
    name AS client_name,
    provider,
    client_id,
    client_secret AS sealed_client_secret
FROM oauth_clients
ORDER BY name;
