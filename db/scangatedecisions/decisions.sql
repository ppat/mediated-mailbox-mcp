-- name: GateDecisionCount :one
-- The scan gate's decisions of one kind since a time, the skips on the system endpoint's corpus block
-- (docs/UI.md section 8.1).
SELECT count(*) AS decisions
FROM scan_gate_decisions AS d
WHERE d.account_id = @account_id AND d.decision = @decision AND d.decided_at >= @since;
