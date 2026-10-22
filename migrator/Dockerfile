# The migrator, the migration step's image (ADR-0048, ADR-0067). It holds goose, built from source without the drivers
# of every other database, and the migration chain. Build it from the repository root.
#
#   docker build --file migrator/Dockerfile .
#
# It reads the connection string from GOOSE_DBSTRING, which the deployment supplies with the migration
# role's credentials. The default command applies every pending migration.

FROM public.ecr.aws/docker/library/golang:1.27.3-trixie@sha256:fbb7ae40b62ff8afe31662df570d39b4f5052ac32b90dae8e60d7657a20a74f4 AS goose
# renovate: datasource=go depName=github.com/pressly/goose/v3
ARG GOOSE_VERSION=v3.28.0
# no_mssql also excludes azuresql, and the tag is listed anyway so the list reads as goose documents it.
RUN CGO_ENABLED=0 go install -trimpath -ldflags="-s -w" \
      -tags="no_azuresql no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb" \
      github.com/pressly/goose/v3/cmd/goose@${GOOSE_VERSION}

FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3
COPY --from=goose /go/bin/goose /goose
COPY db/migrations /migrations
ENV GOOSE_DRIVER=postgres \
    GOOSE_MIGRATION_DIR=/migrations
ENTRYPOINT ["/goose"]
CMD ["up"]
