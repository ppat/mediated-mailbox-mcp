-- name: RecordMaskingEvent :exec
-- Records one mask applied to a message's subject as the subject was masked, naming the rule and tier
-- that detected what was masked and never the text (ADR-0003), and the scanner version and
-- configuration revision the masking ran under (ADR-0096).
INSERT INTO masking_events (account_id, message_id, field, rule_id, tier, scanner_version, scanner_revision)
VALUES (@account_id, @message_id, 'subject', @rule_id, @tier, @scanner_version, @scanner_revision);
