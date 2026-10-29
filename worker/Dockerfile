# The worker's image (ADR-0049). It holds the worker's binary and nothing else, which runs every job kind (ADR-0117).
# Build it from the repository root.
#
#   docker build --file worker/Dockerfile .

FROM public.ecr.aws/docker/library/golang:1.27.3-trixie@sha256:c65ecfe613b539ac74c62a4c3b2c27e8f8a6076384c8f27e8a3c0f3be17cc385 AS server
WORKDIR /src
COPY go.mod go.sum ./
COPY content content
COPY core core
COPY db db
COPY executioncontext executioncontext
COPY process process
COPY provider provider
COPY ratelimit ratelimit
COPY worker/main.go worker/main.go
COPY worker/app worker/app
COPY worker/internal worker/internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/mediated-mailbox-worker ./worker

FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3
COPY --from=server /out/mediated-mailbox-worker /mediated-mailbox-worker
ENTRYPOINT ["/mediated-mailbox-worker"]
