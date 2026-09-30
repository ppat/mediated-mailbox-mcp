-- +goose Up
-- The UI's reads of a run's failure count, its failures per disposition and the runs that recovered
-- them, and its timeline's detail sit in db/jobruns, the tables' own subsection (ADR-0066), which the
-- mediator's list also admits. So the mediator's role is granted the columns those statements read
-- beyond 00011's, under ADR-0075's three lines, which accept reads beyond them. The mediator writes
-- neither table, and none of these columns is one a record forbids it. The failures' reads that
-- read the stored sender class sit in db/jobruns/classification, which the mediator's list does not
-- admit (ADR-0002).
GRANT SELECT (account_id, run_id, disposition, recovered_by) ON job_run_failures
TO mediated_mailbox_mediate;
GRANT SELECT (seq, detail) ON job_run_events TO mediated_mailbox_mediate;
