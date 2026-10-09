# The Heuristics Job's image (ADR-0049). It holds the propose binary and nothing else. Build it from the repository root.
#
#   docker build --file propose/Dockerfile .

FROM public.ecr.aws/docker/library/golang:1.27.2-trixie@sha256:e58d6f83b3416618d8bcac2b3dde1b7f7e3c4a77d25e88637f8bbae81536c48d AS server
WORKDIR /src
COPY go.mod go.sum ./
COPY content content
COPY core core
COPY db db
COPY provider provider
COPY ratelimit ratelimit
COPY propose/main.go propose/main.go
COPY propose/internal propose/internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/mediated-mailbox-propose ./propose

FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3
COPY --from=server /out/mediated-mailbox-propose /mediated-mailbox-propose
ENTRYPOINT ["/mediated-mailbox-propose"]
