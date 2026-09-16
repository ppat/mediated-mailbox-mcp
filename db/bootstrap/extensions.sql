-- Database-level setup, run by a superuser in the application database before the migration chain.
-- vector is not a trusted extension, so the migration role cannot create it. The chain's own
-- CREATE EXTENSION IF NOT EXISTS statements then succeed without doing anything.
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS vector;
