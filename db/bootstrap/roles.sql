-- The test harness's cluster-level setup, run once per cluster before the migration chain by a role
-- allowed to create roles. Roles belong to the whole cluster, PostgreSQL has no CREATE ROLE IF NOT
-- EXISTS, and the chain refuses the DO block that would stand in for it, so roles cannot be created by
-- a migration.
--
-- The project names no database user. A deployment's platform creates the same two users by its own
-- means, under names the deployment chooses, and gives each its name as configuration (ADR-0143).
-- These are the test harness's names. The application user's differs from the placeholder the chain's
-- grants name, so a chain applied without the migrator's replacement of it fails at its first grant
-- (ADR-0048). Credentials are not set here. The platform, or the test harness, gives each user its own.

-- Owns the schema and runs the migration chain (ADR-0048).
CREATE ROLE mediated_mailbox_migrator LOGIN;

-- The one user the API server, the UI and every job kind of the worker connect as (ADR-0143). It owns
-- nothing, holds no attribute beyond LOGIN and belongs to no other role, so it holds only the grants the
-- chain gives it.
CREATE ROLE mediated_mailbox_app LOGIN;
