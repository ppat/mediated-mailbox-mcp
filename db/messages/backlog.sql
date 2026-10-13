-- name: ScanBacklog :one
-- How many of the account's messages wait for their content scan, the scan backlog the system status
-- exposes (ADR-0034, docs/UI.md section 8.8).
SELECT count(*) AS pending
FROM messages AS m
WHERE m.account_id = current_setting('app.account') AND m.scan_state = 'pending';
