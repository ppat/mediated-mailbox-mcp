-- name: BodyGate :many
-- What the Redaction Gate decides a body request from, read at fetch time (ADR-0002): the sender it
-- classifies again under the policy in force, the content flags and scan state the index stores, and
-- the content rules that set the flags, which the request's audit row names. Never the stored sender
-- class. No row means the account holds no such message.
SELECT
    m.from_email,
    m.content_flags,
    m.scan_state,
    m.rule_ids
FROM messages AS m
WHERE m.account_id = @account_id AND m.message_id = @message_id;
