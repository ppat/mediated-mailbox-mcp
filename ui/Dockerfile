# The UI's image (ADR-0049). It holds the UI binary and nothing else, with the browser bundle embedded in it.
# Build it from the repository root.
#
#   docker build --file ui/Dockerfile .

FROM oven/bun:1.4.2-slim@sha256:cb3bbbb08e13a4a2ff400f24c7a2a1d5efa83f6ef8544d52d95a519631e2fc61 AS bundle
WORKDIR /src/ui/browser
COPY ui/browser/package.json ui/browser/bun.lock ui/browser/bunfig.toml ./
# The bundle needs only the production dependencies, and no install script runs.
RUN bun install --production --ignore-scripts
COPY ui/browser/tsconfig.json ./
COPY ui/browser/scripts/build.ts scripts/build.ts
COPY ui/browser/src src
RUN bun scripts/build.ts

FROM golang:1.27.1-trixie@sha256:9baa6b4187bbb98d240372a8a235ac0bb6b5ddd52bba1431dc2f7c0705862728 AS server
WORKDIR /src
COPY go.mod go.sum ./
COPY core core
COPY db db
COPY provider provider
COPY ratelimit ratelimit
COPY sanitize sanitize
COPY ui/main.go ui/main.go
COPY ui/internal ui/internal
COPY --from=bundle /src/ui/browser/dist ui/browser/dist
# The embed pattern also compiles over the placeholder alone, which is how Go tooling works on a fresh clone.
# An image built that way would serve no UI, so the build stops here unless the bundle stage wrote files.
RUN if [ -z "$(find ui/browser/dist -type f ! -name .gitkeep)" ]; then \
      echo "ui/browser/dist holds no bundle, only its placeholder. The bundle stage did not build." >&2; \
      exit 1; \
    fi
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/mediated-mailbox-ui ./ui

FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3
COPY --from=server /out/mediated-mailbox-ui /mediated-mailbox-ui
ENTRYPOINT ["/mediated-mailbox-ui"]
