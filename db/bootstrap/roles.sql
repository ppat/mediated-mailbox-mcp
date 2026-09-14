-- Cluster-level setup, run once per cluster by a superuser before the migration chain. Roles belong to
-- the whole cluster, PostgreSQL has no CREATE ROLE IF NOT EXISTS, and the chain refuses the DO block
-- that would stand in for it, so roles cannot be created by a migration.
--
-- A platform provides the same roles by its own means. The test harness runs this file as written.
-- Credentials are not set here. The platform, or the test harness, gives the migration role its own.
--
-- The runtime roles belong in this file too. How many there are and which component runs as which is
-- not decided, so none is created yet.

-- Owns the schema and runs the migration chain (ADR-0048).
CREATE ROLE mediated_mailbox_migrate LOGIN;
