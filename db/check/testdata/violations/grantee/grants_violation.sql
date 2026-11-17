-- +goose Up
-- A migration naming roles other than the placeholder, as grantees and elsewhere, and naming the
-- placeholder where db/migrate would replace something other than a grantee. The grants and revokes to
-- the placeholder spelled as db/migrate matches it, a default privilege's included, are admitted, and
-- so are roles a keyword names outside a grantee.

GRANT SELECT ON fixture_senders TO application_user;
GRANT SELECT, UPDATE (message_count) ON fixture_senders, fixture_log TO application_user;
REVOKE UPDATE (message_count) ON fixture_senders FROM application_user;
ALTER DEFAULT PRIVILEGES GRANT SELECT ON TABLES TO application_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE INSERT ON TABLES FROM application_user;
CREATE TABLE fixture_application_users (x_application_user int, application_user_id int);
ALTER TABLE fixture_log OWNER TO CURRENT_USER;

-- want grantee
GRANT SELECT ON fixture_senders TO mediated_mailbox_app;

-- want grantee
GRANT SELECT ON fixture_senders TO PUBLIC;

-- want grantee
REVOKE SELECT ON fixture_senders FROM CURRENT_USER;

-- want grantee
GRANT SELECT ON fixture_senders TO application_user, mediated_mailbox_app;

-- want grantee
ALTER DEFAULT PRIVILEGES GRANT INSERT ON TABLES TO mediated_mailbox_app;

-- want grantee
ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE SELECT ON TABLES FROM PUBLIC;

-- A grant of one role to another is refused, and so is the role it names.
-- want grantee
-- want grantee
GRANT pg_read_all_data TO mediated_mailbox_app;

-- A role named outside a grantee position, once for each form a migration might take.
-- want grantee
ALTER TABLE fixture_log OWNER TO mediated_mailbox_migrator;

-- want grantee
ALTER DEFAULT PRIVILEGES FOR ROLE mediated_mailbox_migrator GRANT SELECT ON TABLES TO application_user;

-- want grantee
CREATE POLICY fixture_policy ON fixture_senders TO mediated_mailbox_app USING (true);

-- want grantee
CREATE SCHEMA fixture_schema AUTHORIZATION mediated_mailbox_migrator;

-- The placeholder outside a grantee position is refused twice, as a named role and as the
-- placeholder standing where it is no grantee, since the application user owns nothing.
-- want grantee
-- want grantee
ALTER TABLE fixture_log OWNER TO application_user;

-- A spelling the replacement would miss is refused twice, once as a grantee other than the
-- placeholder and once as the placeholder standing where it is no grantee.
-- want grantee
-- want grantee
GRANT SELECT ON fixture_senders TO APPLICATION_USER;

-- want grantee
-- want grantee
GRANT SELECT ON fixture_senders TO "application_user";

-- want grantee
SELECT 'application_user';

-- want grantee
CREATE TABLE application_user (id int);


-- A statement that creates, renames or drops a role, or switches the session to one, once for each
-- form, a role given as a string included. A drop names its role, which the rule on named roles
-- refuses.
-- want grantee
CREATE ROLE fixture_extra;

-- want grantee
CREATE USER fixture_extra_user;

-- want grantee
CREATE GROUP fixture_extra_group;

-- want grantee
DROP ROLE mediated_mailbox_extra;

-- want grantee
ALTER ROLE mediated_mailbox_extra RENAME TO mediated_mailbox_other;

-- want grantee
SET ROLE mediated_mailbox_app;

-- want grantee
SET LOCAL ROLE 'mediated_mailbox_migrator';

-- want grantee
RESET ROLE;

-- want grantee
SET SESSION AUTHORIZATION mediated_mailbox_app;

-- want grantee
RESET SESSION AUTHORIZATION;

-- want grantee
-- A comment naming application_user, which the replacement would rewrite.
