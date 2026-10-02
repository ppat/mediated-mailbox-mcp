-- name: ReplaceSealedClient :execrows
-- Writes a provider's re-sealed client secret only if the stored bytes are still the ones delta sync
-- last read, so a client the operator set up again is never put back (ADR-0089, ADR-0092). A write
-- that finds other bytes changes no row.
UPDATE oauth_clients
SET client_secret = @client_secret
WHERE provider = @provider AND client_secret = @known;
