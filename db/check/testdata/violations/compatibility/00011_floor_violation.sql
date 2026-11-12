-- +goose Up
-- A migration raising the compatibility floor in the one form the check accepts, to its own version,
-- beside the other statements the check admits on the floor's table, a read and a grant or a revoke
-- of SELECT alone, and statements of many kinds that name the table otherwise, each of which it
-- refuses.

-- The accepted form, so the file is declared and reports no compatibility-undeclared.
UPDATE schema_compatibility SET oldest_chain_version = greatest(oldest_chain_version, 11);
UPDATE schema_compatibility SET oldest_chain_version = greatest(oldest_chain_version, 1);

-- A grant and a revoke of SELECT alone, and reads, are admitted.
GRANT SELECT (oldest_chain_version) ON schema_compatibility TO mediated_mailbox_organize;
REVOKE SELECT (oldest_chain_version) ON schema_compatibility FROM mediated_mailbox_organize;
CREATE TABLE fixture_floor_copy AS SELECT oldest_chain_version FROM schema_compatibility;
SELECT oldest_chain_version FROM schema_compatibility WHERE oldest_chain_version > 0;

-- want floor-statement
UPDATE schema_compatibility SET oldest_chain_version = 2;

-- want floor-statement
UPDATE schema_compatibility SET oldest_chain_version = greatest(oldest_chain_version, 12);

-- want floor-statement
UPDATE schema_compatibility SET oldest_chain_version = greatest(oldest_chain_version, 0);

-- want floor-statement
UPDATE schema_compatibility SET oldest_chain_version = least(oldest_chain_version, 11);

-- want floor-statement
UPDATE schema_compatibility SET oldest_chain_version = greatest(11, oldest_chain_version);

-- want floor-statement
UPDATE schema_compatibility SET oldest_chain_version = greatest(oldest_chain_version, 5, 11);

-- want floor-statement
UPDATE schema_compatibility SET oldest_chain_version = greatest(oldest_chain_version, 11)
WHERE oldest_chain_version < 11;

-- want floor-statement
INSERT INTO schema_compatibility (oldest_chain_version) VALUES (11);

-- want floor-statement
DELETE FROM schema_compatibility;

-- want floor-statement
ALTER TABLE schema_compatibility ADD COLUMN note text;

-- want floor-statement
DROP TABLE schema_compatibility;

-- want floor-statement
DROP TABLE IF EXISTS public.schema_compatibility;

-- want floor-statement
ALTER TABLE schema_compatibility RENAME TO schema_compatibility_old;

-- want floor-statement
TRUNCATE schema_compatibility;

-- want floor-statement
WITH lowered AS (
    UPDATE schema_compatibility SET oldest_chain_version = 0 RETURNING oldest_chain_version
)
SELECT oldest_chain_version FROM lowered;

-- want floor-statement
CREATE RULE floor_kept AS ON UPDATE TO schema_compatibility DO INSTEAD NOTHING;

-- want floor-statement
MERGE INTO schema_compatibility AS f USING (SELECT 0 AS v) AS s ON true
WHEN MATCHED THEN UPDATE SET oldest_chain_version = s.v;

-- want floor-statement
ALTER TABLE schema_compatibility SET SCHEMA fixture_elsewhere;

-- want floor-statement
COPY schema_compatibility FROM STDIN;

-- want floor-statement
DROP TABLE fixture_senders, schema_compatibility;

-- want floor-statement
SELECT oldest_chain_version FROM schema_compatibility FOR UPDATE;

-- want floor-statement
CREATE POLICY fixture_floor_hidden ON schema_compatibility USING (false);

-- want floor-statement
GRANT UPDATE (oldest_chain_version) ON schema_compatibility TO mediated_mailbox_api;

-- want floor-statement
GRANT ALL ON schema_compatibility TO mediated_mailbox_api;

-- want floor-statement
GRANT SELECT, INSERT ON schema_compatibility TO mediated_mailbox_api;

-- want floor-statement
REVOKE ALL ON schema_compatibility FROM mediated_mailbox_api;

-- want floor-statement
ALTER TABLE fixture_senders RENAME TO schema_compatibility;
