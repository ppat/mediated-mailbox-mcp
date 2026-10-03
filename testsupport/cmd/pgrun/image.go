package main

// image is the PostgreSQL image every test run starts. It carries the extensions the schema needs,
// which the official image lacks for vector. Renovate updates the tag and digest through the custom
// manager in .github/renovate.json that matches this line.
const image = "pgvector/pgvector:0.8.7-pg18-trixie@sha256:9d9c930220cb9bf2f956d10a8f909cf9973d9672ca0278aef2c1e6facccad2e0"
