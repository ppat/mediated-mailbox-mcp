-- +goose Up
-- A scan verdict records the revision of the scanner configuration it was made under beside the
-- scanner version, so a change to the vocabulary or the tuning marks rows stale the way a version
-- change does (ADR-0009, ADR-0016).
ALTER TABLE messages ADD COLUMN scanner_revision text;

-- Backfill's second pass writes what its statements write (ADR-0075, ADR-0017). They sit in
-- db/messages/scan and db/scangatedecisions/record, which only backfill's list admits, and in
-- db/senders and db/accountstate/completion, which only backfill's list admits too.
--
-- Pass 2 reads the messages waiting for a scan with the inputs the scan gate reads, each joined to
-- its sender's volume and prior hits (ADR-0093). It records each message's scan verdict or skip
-- state. The delisting transition reads the domains of the messages stored as restricted or skipped
-- as restricted, and marks those messages of a domain the policy now classifies normal back to a
-- normal sender class and pending scan (ADR-0037).
GRANT SELECT (subject_masked, size_bytes, scan_state),
UPDATE (
    sender_class,
    scan_state,
    content_flags,
    rule_ids,
    scanned_at,
    scanner_version,
    scanner_revision
) ON messages TO mediated_mailbox_backfill;

-- Every gate decision is recorded with its reason, and a message decided again after the delisting
-- transition has its decision replaced (ADR-0093). The upsert reads the conflict columns and the
-- values it replaces them with.
GRANT SELECT (account_id, message_id, decision, reason),
INSERT (account_id, message_id, decision, reason),
UPDATE (decision, reason, decided_at) ON scan_gate_decisions TO mediated_mailbox_backfill;

-- A verdict carrying a content flag adds to its sender's prior hits, which the gate reads.
GRANT SELECT (scan_hit_count), UPDATE (scan_hit_count) ON senders TO mediated_mailbox_backfill;

-- Pass 2 sets its completion flag when it ends.
GRANT UPDATE (backfill_pass2_complete) ON account_state TO mediated_mailbox_backfill;
