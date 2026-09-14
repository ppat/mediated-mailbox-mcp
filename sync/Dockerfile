# Delta sync's image (ADR-0049). It holds the sync binary and nothing else. Build it from the repository root.
#
#   docker build --file sync/Dockerfile .

FROM golang:1.27.1-trixie@sha256:9baa6b4187bbb98d240372a8a235ac0bb6b5ddd52bba1431dc2f7c0705862728 AS server
WORKDIR /src
COPY go.mod go.sum ./
COPY core core
COPY db db
COPY provider provider
COPY ratelimit ratelimit
COPY sync/main.go sync/main.go
COPY sync/internal sync/internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/mediated-mailbox-sync ./sync

FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3
COPY --from=server /out/mediated-mailbox-sync /mediated-mailbox-sync
ENTRYPOINT ["/mediated-mailbox-sync"]
