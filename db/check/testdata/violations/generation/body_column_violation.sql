-- A metadata query reading a body column, which the schema does not hold (ADR-0016). Generating the
-- data-access code must fail on that column, so no result type can carry a body (ADR-0047). The
-- generation check places this file in a copy of the test library and runs sqlc generate there. The
-- statement names no other column, so a column renamed to body cannot leave it failing on another.
-- want generation
-- name: GetMessageBody :many
SELECT body
FROM messages;
