package main

// image is the PostgreSQL image every test run starts, the official image, which carries the one
// extension the schema needs, pulled from mirror.gcr.io rather than Docker Hub. Renovate updates the
// tag and digest through the custom manager in .github/renovate.json that matches this line.
const image = "mirror.gcr.io/library/postgres:18.6-trixie@sha256:74935e72241653ca55e0414067e6d8763aceb8a810eb51b452253ec3dcfc4336"
