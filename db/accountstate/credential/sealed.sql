-- name: Sealed :many
-- The account's sealed credential, as its state row holds it. No row means the account has no state
-- row and is not connected, and a null credential means it holds none (ADR-0091).
SELECT credential
FROM account_state
WHERE account_id = @account_id;

-- name: ReplaceSealed :execrows
-- Writes a sealed credential back only if the stored bytes are still the ones the process last read
-- or wrote, so a value the operator replaced is never put back (ADR-0089). A write that finds other
-- bytes changes no row.
UPDATE account_state
SET credential = @credential
WHERE account_id = @account_id AND credential = @known;
