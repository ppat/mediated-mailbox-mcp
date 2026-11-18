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

-- name: SealedClient :one
-- The named client's sealed secret, which the UI's client-secret package opens for a consent's code
-- exchange (ADR-0081). The table belongs to no account, and a name no client holds reads no row.
SELECT client_secret AS sealed_client_secret
FROM oauth_clients
WHERE name = @client_name;
