-- name: RecordDecision :exec
-- Records the scan gate's decision on one message with its reason, scans as well as skips
-- (ADR-0093). A message decided again, as one the delisting transition returned to pending scan is,
-- has its decision replaced.
INSERT INTO scan_gate_decisions AS d (account_id, message_id, decision, reason)
VALUES (@account_id, @message_id, @decision, @reason)
ON CONFLICT (account_id, message_id) DO UPDATE
    SET decision = excluded.decision, reason = excluded.reason, decided_at = now();
