-- +goose Up
-- The one extension the schema needs (ADR-0016). citext is a trusted extension, so the migration role,
-- owning the database, creates it, and no step inside the application database needs a superuser
-- (ADR-0048).
CREATE EXTENSION IF NOT EXISTS citext;
