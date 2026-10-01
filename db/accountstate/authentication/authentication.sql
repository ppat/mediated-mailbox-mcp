-- name: RecordAuthentication :exec
-- Records an authentication attempt the account's provider adapter reported, at the end of a
-- unit of work, only when the row holds no attempt or an older one, so the latest attempt of
-- every deployable that records it wins and an older one changes nothing (ADR-0097, ADR-0016).
UPDATE account_state
SET
    last_auth_at = @attempted_at,
    last_auth_outcome = @outcome
WHERE
    account_id = @account_id
    AND (last_auth_at IS NULL OR last_auth_at < @attempted_at);
