# The API server's image (ADR-0049). It holds the api binary and nothing else. Build it from the repository root.
#
#   docker build --file api/Dockerfile .

FROM public.ecr.aws/docker/library/golang:1.27.4-trixie@sha256:f18c3d73a184fb0582828510722e723b63a93f7188d11dd223d9be6f6880e65d AS server
WORKDIR /src
COPY go.mod go.sum ./
COPY content content
COPY core core
COPY credential credential
COPY db db
COPY process process
COPY provider provider
COPY api/main.go api/main.go
COPY api/app api/app
COPY api/internal api/internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/mediated-mailbox-api ./api

FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3
COPY --from=server /out/mediated-mailbox-api /mediated-mailbox-api
ENTRYPOINT ["/mediated-mailbox-api"]
