-- name: RateStatus :one
-- The rate state the UI's jobs screen and event stream show, the current rate, the target, the cap,
-- the backoff and the last throttle, and each priority class's reservation and use as the limiter
-- records them (ADR-0024, ADR-0025, docs/UI.md section 8.3).
SELECT
    r.current_rate,
    r.target_rate,
    r.hard_cap,
    r.backoff_until,
    r.last_throttle_at,
    r.classes
FROM rate_state AS r
WHERE r.account_id = @account_id;
