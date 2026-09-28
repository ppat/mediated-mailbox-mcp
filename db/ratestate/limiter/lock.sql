-- name: LockRateState :exec
-- Holds the account's rate state until the transaction ends, so the issuer and the controller take
-- turns on one account (ADR-0025). The key is derived from the account, and two accounts whose keys
-- collide only take turns with each other.
SELECT pg_advisory_xact_lock(hashtextextended(@account_id::text, 0));
