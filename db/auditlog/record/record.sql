-- name: RecordBodyDecision :exec
-- Records one body request's decision, a serve as READ_BODY and a denial as DENY_BODY, with what it
-- rests on in typed columns. They are the stage that decided, the reason the client receives, the
-- sender class with the policy rule behind it and that rule's scope, the stored content flags with the
-- scanner rules behind them, the scan state, and the serve-time check's rules and scanner where that
-- check ran. The row never holds any of the body's text (ADR-0002, ADR-0016). The audit log is append-only, so this is
-- the only write it takes.
INSERT INTO audit_log (
    account_id, actor, action, message_id, stage, reason, sender_class, class_rule_id, class_rule_scope,
    content_flags, content_rule_ids, serve_rule_ids, scan_state, scanner_version, scanner_revision
)
VALUES (
    @account_id, @actor, @action, @message_id, @stage, @reason, @sender_class, @class_rule_id, @class_rule_scope,
    @content_flags, @content_rule_ids, @serve_rule_ids, @scan_state, @scanner_version, @scanner_revision
);
