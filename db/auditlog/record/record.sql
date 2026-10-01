-- name: RecordBodyDecision :exec
-- Records one body request's decision, a serve as READ_BODY and a denial as DENY_BODY, with the
-- sensitivity it was decided under and the rules that decided it, and never any of the body's text
-- (ADR-0002, ADR-0016). The audit log is append-only, so this is the only write it takes.
INSERT INTO audit_log (account_id, actor, action, message_id, sensitivity, rule_ids)
VALUES (@account_id, @actor, @action, @message_id, @sensitivity, @rule_ids);
