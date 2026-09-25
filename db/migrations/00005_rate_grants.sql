-- +goose Up
-- The rate state gains the token bucket, the count of throttles since the last success, the instant
-- each priority class last asked, the latest grant and the latency windows, and every grant of the
-- last second gets a row of its own, which is also the live lease it issued (ADR-0016, ADR-0024,
-- ADR-0025). A row per grant replaces the one leased amount and one expiry, because every worker
-- holds a lease of its own and the one-second window counts every grant whole.
--
-- The issuer recomputes the target and the hard cap from the provider's declared ceiling on every
-- issue and never reads target_rate, hard_cap, baseline_p50_ms or classes back. It writes them only
-- for display, so no write to the row can raise the cap.

ALTER TABLE rate_state
DROP COLUMN leased_tokens,
DROP COLUMN lease_expires_at,
-- Throttles since the last success, for the backoff.
ADD COLUMN throttles integer NOT NULL DEFAULT 0,
-- The token bucket's level, and the instant that level was reached.
ADD COLUMN bucket_level real NOT NULL DEFAULT 0,
ADD COLUMN bucket_filled_at timestamptz,
-- When each class last asked, interactive, sync and batch in that order.
ADD COLUMN class_asked_at timestamptz[],
-- The instant of the latest grant.
ADD COLUMN last_granted_at timestamptz,
-- The current one-minute latency window, its samples in milliseconds, and the last ten window
-- medians.
ADD COLUMN latency_window_start timestamptz,
ADD COLUMN latency_samples integer[],
ADD COLUMN latency_medians real[];

-- Every grant of the last second. The issuer writes a grant in the same transaction as the bucket's
-- level and deletes rows more than a second old. A lost row widens issuance without the rules seeing
-- it (ADR-0024). A lease expires one second after it is issued.
CREATE TABLE rate_grants (
    grant_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id text NOT NULL REFERENCES accounts,
    -- interactive, sync or batch.
    class text NOT NULL,
    tokens real NOT NULL,
    issued_at timestamptz NOT NULL
);
CREATE INDEX ON rate_grants (account_id, issued_at);

ALTER TABLE rate_grants ENABLE ROW LEVEL SECURITY;
CREATE POLICY rate_grants_account ON rate_grants
USING (account_id = current_setting('app.account'));

-- The rate limiter is a shared library, and its statements run under the role of each deployable
-- that spends from the budget (ADR-0075). Those are the mediator, backfill, delta sync and the
-- reorganization workload. Each gets exactly what the statements in db/ratestate need. They insert
-- the account's rate state when it has none, read it, and update every column but the account. They
-- read, insert and delete grants, and move a grant stamped ahead of the clock back to it.
GRANT SELECT, INSERT (account_id, current_rate, target_rate, hard_cap),
UPDATE (
    current_rate,
    target_rate,
    hard_cap,
    baseline_p50_ms,
    last_throttle_at,
    backoff_until,
    throttles,
    bucket_level,
    bucket_filled_at,
    class_asked_at,
    last_granted_at,
    latency_window_start,
    latency_samples,
    latency_medians,
    classes,
    updated_at
) ON rate_state
TO mediated_mailbox_mediate, mediated_mailbox_backfill, mediated_mailbox_sync, mediated_mailbox_organize;
GRANT SELECT, INSERT (account_id, class, tokens, issued_at), UPDATE (issued_at), DELETE ON rate_grants
TO mediated_mailbox_mediate, mediated_mailbox_backfill, mediated_mailbox_sync, mediated_mailbox_organize;
