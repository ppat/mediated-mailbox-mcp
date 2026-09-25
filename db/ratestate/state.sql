-- name: InsertRateState :exec
-- Gives an account its rate state the first time it spends. An account that already has one keeps it.
INSERT INTO rate_state (account_id, current_rate, target_rate, hard_cap)
VALUES (@account_id, @current_rate, @target_rate, @hard_cap)
ON CONFLICT (account_id) DO NOTHING;

-- name: RateState :one
-- The rate state the rules decide from, with every instant in Unix milliseconds and zero for an instant
-- never set. target_rate, hard_cap, baseline_p50_ms and classes are left out, because they are written
-- only for display and the rules recompute them (ADR-0024).
SELECT
    current_rate,
    throttles,
    bucket_level,
    coalesce(floor(extract(EPOCH FROM backoff_until) * 1000), 0)::bigint AS backoff_until_ms,
    coalesce(floor(extract(EPOCH FROM bucket_filled_at) * 1000), 0)::bigint AS bucket_filled_ms,
    coalesce(floor(extract(EPOCH FROM class_asked_at[1]) * 1000), 0)::bigint AS interactive_asked_ms,
    coalesce(floor(extract(EPOCH FROM class_asked_at[2]) * 1000), 0)::bigint AS sync_asked_ms,
    coalesce(floor(extract(EPOCH FROM class_asked_at[3]) * 1000), 0)::bigint AS batch_asked_ms,
    coalesce(floor(extract(EPOCH FROM last_granted_at) * 1000), 0)::bigint AS last_granted_ms,
    coalesce(floor(extract(EPOCH FROM latency_window_start) * 1000), 0)::bigint AS latency_window_start_ms,
    coalesce(latency_samples, '{}')::integer[] AS latency_samples,
    coalesce(latency_medians, '{}')::real[] AS latency_medians
FROM rate_state
WHERE account_id = @account_id;

-- name: UpdateIssuance :exec
-- Stores what issuance keeps after one request, whether or not it was granted. A granted_ms of zero
-- leaves the latest grant as it was.
UPDATE rate_state
SET
    target_rate = @target_rate,
    hard_cap = @hard_cap,
    bucket_level = @bucket_level,
    bucket_filled_at = timestamptz 'epoch' + (@bucket_filled_ms::bigint) * interval '1 millisecond',
    class_asked_at = ARRAY[
        timestamptz 'epoch' + nullif((@interactive_asked_ms::bigint), 0) * interval '1 millisecond',
        timestamptz 'epoch' + nullif((@sync_asked_ms::bigint), 0) * interval '1 millisecond',
        timestamptz 'epoch' + nullif((@batch_asked_ms::bigint), 0) * interval '1 millisecond'
    ],
    last_granted_at = coalesce(
        timestamptz 'epoch' + nullif((@granted_ms::bigint), 0) * interval '1 millisecond', last_granted_at
    ),
    classes = @classes,
    updated_at = timestamptz 'epoch' + (@now_ms::bigint) * interval '1 millisecond'
WHERE account_id = @account_id;

-- name: UpdateController :exec
-- Stores the controller's state after a call's outcome. A throttled_ms of zero leaves the latest
-- throttle as it was, and a baseline of zero stores none.
UPDATE rate_state
SET
    current_rate = @current_rate,
    target_rate = @target_rate,
    hard_cap = @hard_cap,
    backoff_until = timestamptz 'epoch' + nullif((@backoff_until_ms::bigint), 0) * interval '1 millisecond',
    throttles = @throttles,
    last_throttle_at = coalesce(
        timestamptz 'epoch' + nullif((@throttled_ms::bigint), 0) * interval '1 millisecond', last_throttle_at
    ),
    latency_window_start
    = timestamptz 'epoch' + nullif((@latency_window_start_ms::bigint), 0) * interval '1 millisecond',
    latency_samples = @latency_samples::integer[],
    latency_medians = @latency_medians::real[],
    baseline_p50_ms = nullif(@baseline_ms::real, 0),
    updated_at = timestamptz 'epoch' + (@now_ms::bigint) * interval '1 millisecond'
WHERE account_id = @account_id;
