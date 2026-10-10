-- Cluster-level setup, run once per cluster before the migration chain by a role allowed to create
-- roles, the one privileged step a deployment takes. Roles belong to the whole cluster, PostgreSQL has
-- no CREATE ROLE IF NOT EXISTS, and the chain refuses the DO block that would stand in for it, so
-- roles cannot be created by a migration. The chain's grants name these roles literally.
--
-- A platform provides the same roles by its own means. The test harness runs this file as written.
-- Credentials are not set here. The platform, or the test harness, gives each role its own.

-- Owns the schema and runs the migration chain (ADR-0048).
CREATE ROLE mediated_mailbox_migrate LOGIN;

-- One runtime role for the mediator, one for the UI, and one for each job kind of the worker,
-- backfill, delta sync, reorg apply and rollback, and the heuristics run (ADR-0118). None owns
-- anything, bypasses row-level security or belongs to another role, so each holds only the grants the
-- chain gives it.
CREATE ROLE mediated_mailbox_mediate LOGIN;
CREATE ROLE mediated_mailbox_backfill LOGIN;
CREATE ROLE mediated_mailbox_sync LOGIN;
CREATE ROLE mediated_mailbox_organize LOGIN;
CREATE ROLE mediated_mailbox_propose LOGIN;
CREATE ROLE mediated_mailbox_ui LOGIN;
