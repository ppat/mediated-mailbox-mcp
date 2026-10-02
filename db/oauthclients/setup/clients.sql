-- name: SetupClients :many
-- Every stored OAuth client's name, provider, client identifier and project ID, which the
-- installation endpoint lists and client setup checks a name and an identifier against. It reads no
-- secret. The role that runs it may read one only for the UI's client-secret part, which reads it
-- through its own statement (ADR-0081, ADR-0084, ADR-0106).
SELECT
    name AS client_name,
    provider,
    client_id,
    project_id
FROM oauth_clients
ORDER BY name;

-- name: AddClient :exec
-- Stores a new client under its name with its secret sealed to the row its name keys (ADR-0081,
-- ADR-0088). The table's keys refuse a name another client holds and a client identifier the
-- provider's other client holds (ADR-0106).
INSERT INTO oauth_clients (name, provider, client_id, client_secret, project_id)
VALUES (@client_name, @provider, @client_id, @client_secret, @project_id);

-- name: ReplaceClient :execrows
-- Replaces the named client's identifier, secret and project ID, unconditionally, since the operator's
-- write is the newest (ADR-0089). No row means no client of the provider holds the name.
UPDATE oauth_clients
SET
    client_id = @client_id,
    client_secret = @client_secret,
    project_id = @project_id
WHERE name = @client_name AND provider = @provider;

-- name: RemoveClient :execrows
-- Removes the named client. An account that connects through it refuses the delete by its reference,
-- so a client in use is never removed (ADR-0106). No row means no client of the provider holds the
-- name.
DELETE FROM oauth_clients
WHERE name = @client_name AND provider = @provider;

-- name: ClientIdentity :many
-- The named client's provider and identifier, which finishing a consent reads in the transaction that
-- writes the account, to refuse a client replaced or removed since the attempt started (docs/UI.md
-- section 8.12).
SELECT
    provider,
    client_id
FROM oauth_clients
WHERE name = @client_name;
