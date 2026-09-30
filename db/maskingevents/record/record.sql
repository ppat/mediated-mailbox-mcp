-- name: RecordMaskingEvent :exec
-- Records one mask applied to a message's subject as it entered the index, naming the rule and tier
-- that detected what was masked and never the text (ADR-0003).
INSERT INTO masking_events (account_id, message_id, field, rule_id, tier)
VALUES (@account_id, @message_id, 'subject', @rule_id, @tier);
