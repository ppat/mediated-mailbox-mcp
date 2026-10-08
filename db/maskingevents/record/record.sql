-- name: RecordMaskingEvent :exec
-- Records one mask applied to a message's subject as the subject was masked, naming the rule and tier
-- that detected what was masked and never the text (ADR-0003), and the scanner version and
-- configuration revision the masking ran under (ADR-0120).
INSERT INTO masking_events (account_id, message_id, field, rule_id, tier, scanner_version, scanner_revision)
VALUES (@account_id, @message_id, 'subject', @rule_id, @tier, @scanner_version, @scanner_revision);

-- name: RecordMaskingEvents :exec
-- Records a batch of masks applied to the subjects of the account's messages, one row for each
-- message, rule and tier named in the same position of the three lists, naming never the text
-- (ADR-0003), all under the scanner version and configuration revision given, in one statement, as a
-- backfill run's start masks the stored subjects again in batches (ADR-0120).
INSERT INTO masking_events (account_id, message_id, field, rule_id, tier, scanner_version, scanner_revision)
SELECT
    @account_id,
    e.message_id,
    'subject' AS field,
    e.rule_id,
    e.tier,
    @scanner_version::int AS scanner_version,
    @scanner_revision::text AS scanner_revision
FROM (
    SELECT
        unnest(@message_ids::text[]) AS message_id,
        unnest(@rule_ids::text[]) AS rule_id,
        unnest(@tiers::int[]) AS tier
) AS e;
