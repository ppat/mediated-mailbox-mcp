# The UI's image (ADR-0049). It holds the UI binary and nothing else, with the browser bundle embedded in it.
# Build it from the repository root.
#
#   docker build --file ui/Dockerfile .

FROM mirror.gcr.io/oven/bun:1.4.5-slim@sha256:b780593f3e10918cbdaec3d5a08d52224b9198069a65eb47812085c2ec007643 AS bundle
WORKDIR /src/ui/browser
COPY ui/browser/package.json ui/browser/bun.lock ui/browser/bunfig.toml ./
# The bundle needs only the production dependencies, and no install script runs.
RUN bun install --production --ignore-scripts
COPY ui/browser/tsconfig.json ./
COPY ui/browser/scripts/build.ts scripts/build.ts
COPY ui/browser/src src
RUN bun scripts/build.ts

FROM public.ecr.aws/docker/library/golang:1.27.4-trixie@sha256:b54b2639de8c1bfe538ec5ffc45f29b1932235ac5f6d63c059853646677112d9 AS server
WORKDIR /src
COPY go.mod go.sum ./
COPY core core
COPY credential credential
COPY db db
COPY process process
COPY provider provider
COPY ui/main.go ui/main.go
COPY ui/app ui/app
COPY ui/internal ui/internal
COPY ui/browser/bundle.go ui/browser/bundle.go
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
