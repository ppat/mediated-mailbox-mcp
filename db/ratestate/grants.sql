-- name: Grants :many
-- Every grant the account holds a row for, which the issuer keeps to the last second.
SELECT
    class,
    tokens,
    floor(extract(EPOCH FROM issued_at) * 1000)::bigint AS issued_ms
FROM rate_grants
WHERE account_id = @account_id;

-- name: InsertGrant :exec
INSERT INTO rate_grants (account_id, class, tokens, issued_at)
VALUES (@account_id, @class, @tokens, timestamptz 'epoch' + (@issued_ms::bigint) * interval '1 millisecond');

-- name: DeleteGrantsBefore :exec
-- Deletes the grants issued at or before the cutoff, one second before the clock the issuer read.
DELETE FROM rate_grants
WHERE
    account_id = @account_id
    AND issued_at <= timestamptz 'epoch' + (@cutoff_ms::bigint) * interval '1 millisecond';

-- name: ClampGrants :exec
-- Moves a grant stamped later than the instant issuance has reached back to that instant, as the rules
-- take it (ratelimit/core). It then counts for one second from there, rather than for as long as its
-- stamp lies ahead.
UPDATE rate_grants
SET issued_at = timestamptz 'epoch' + (@now_ms::bigint) * interval '1 millisecond'
WHERE
    account_id = @account_id
    AND issued_at > timestamptz 'epoch' + (@now_ms::bigint) * interval '1 millisecond';
