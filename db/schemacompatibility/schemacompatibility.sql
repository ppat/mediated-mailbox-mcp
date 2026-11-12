-- name: SchemaVersions :one
-- The oldest chain whose binaries may run on the schema, and the newest migration the database has
-- applied, as goose records it, read in full by every role a deployable connects as (ADR-0140).
SELECT
    schema_compatibility.oldest_chain_version,
    (SELECT max(goose_db_version.version_id) FROM goose_db_version)::bigint AS applied
FROM schema_compatibility;
