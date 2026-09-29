-- name: ClockMillis :one
-- The database's clock in Unix milliseconds. It is read in its own statement after the lock is held,
-- because now() is fixed when the transaction starts and a worker that waited on the lock would stamp
-- its lease early (ADR-0025).
SELECT floor(extract(EPOCH FROM clock_timestamp()) * 1000)::bigint AS now_ms;
