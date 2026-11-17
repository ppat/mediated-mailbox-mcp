# The migrator's image, which the migration step runs (ADR-0048, ADR-0049, ADR-0139). It holds the migrator's binary
# and nothing else, and the binary carries the migration chain it applies. Build it from the repository root.
#
#   docker build --file migrator/Dockerfile .

FROM public.ecr.aws/docker/library/golang:1.27.4-trixie@sha256:f18c3d73a184fb0582828510722e723b63a93f7188d11dd223d9be6f6880e65d AS server
WORKDIR /src
COPY go.mod go.sum ./
COPY db db
COPY process process
COPY migrator/main.go migrator/main.go
COPY migrator/app migrator/app
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/mediated-mailbox-migrator ./migrator

FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3
COPY --from=server /out/mediated-mailbox-migrator /mediated-mailbox-migrator
ENTRYPOINT ["/mediated-mailbox-migrator"]
