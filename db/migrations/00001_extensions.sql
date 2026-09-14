-- +goose Up
-- The extensions the schema needs (ADR-0016). vector is not a trusted extension, so only a superuser
-- can create it. The bootstrap in db/bootstrap creates all three before the chain runs, and these
-- statements then succeed without doing anything. They stay so the chain states what it depends on.
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS vector;
