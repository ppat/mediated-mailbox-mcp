-- +goose Up
-- A stored subject records the scanner version and the scanner configuration's revision its masking
-- ran under, beside the pair a scan verdict records, and so does each masking event, so a change of
-- scanner masks the subject again and a reader tells the masks of the subject as it stands from those
-- of a masking it replaced (ADR-0096, ADR-0003).
ALTER TABLE messages ADD COLUMN subject_scanner_version int, ADD COLUMN subject_scanner_revision text;
ALTER TABLE masking_events ADD COLUMN scanner_version int, ADD COLUMN scanner_revision text;

-- The run-start step that returns stale verdicts to pending marks the second pass to start over, and the
-- second pass run that starts clears the mark, so a run resuming a stopped pass reads what was returned
-- to pending before its checkpoint, without anything written to the stopped run (ADR-0096).
ALTER TABLE account_state ADD COLUMN backfill_pass2_restart boolean NOT NULL DEFAULT false;
GRANT SELECT (backfill_pass2_restart), UPDATE (backfill_pass2_restart) ON account_state TO mediated_mailbox_backfill;

-- Backfill writes what its statements write (ADR-0075, ADR-0096). Its first pass adds each message with
-- the pair its subject was masked under, finds whether any stored subject carries another pair, masks
-- such a subject again, and masks whole the stored subject of a message the provider no longer has,
-- reading it first. A backfill run, before its first pass, returns each scanned message whose verdict
-- carries another pair to pending with its verdict cleared, and counts its sender's prior hits again from
-- the flagged verdicts the index holds.
GRANT SELECT (
    subject,
    subject_scanner_version,
    subject_scanner_revision,
    content_flags,
    scanner_version,
    scanner_revision
),
INSERT (subject_scanner_version, subject_scanner_revision),
UPDATE (subject, subject_masked, subject_scanner_version, subject_scanner_revision) ON messages
TO mediated_mailbox_backfill;
GRANT INSERT (scanner_version, scanner_revision) ON masking_events TO mediated_mailbox_backfill;
