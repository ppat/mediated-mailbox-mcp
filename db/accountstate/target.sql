-- name: LoweredTarget :one
-- The lower target the operator set for the account, a fraction of its provider's declared ceiling, or
-- null for none. The rate limiter takes it for the account when a spending process builds it (ADR-0024).
SELECT s.lowered_target_rate
FROM account_state AS s
WHERE s.account_id = @account_id;
