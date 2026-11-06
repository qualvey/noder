# Build the Vue admin panel first so it is embedded into the Go binary.
FROM node:22-bookworm AS frontend

WORKDIR /src/web
RUN corepack enable && corepack prepare pnpm@10.12.1 --activate
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# Build both the HTTP server and the one-shot SQLite importer.
FROM golang:1.27-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
COPY --from=frontend /src/static/ ./static/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/noder ./cmd/server \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/noder-migrate ./cmd/migrate

# The image does not contain the legacy database. Compose mounts it read-only
# into the migration service and stores uploaded files in a named volume.
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=builder /out/noder /usr/local/bin/noder
COPY --from=builder /out/noder-migrate /usr/local/bin/noder-migrate
COPY --from=builder /src/static ./static
COPY --from=builder /src/templates/sing-box.json /app/templates/sing-box.json
COPY --from=builder /src/templates/xray.json /app/templates/xray.json
COPY --from=builder /src/templates/mihomo.yml /app/templates/mihomo.yml
COPY --chown=65532:65532 --from=builder /src/templates/mihomo.yml /var/lib/noder/.keep

ENTRYPOINT ["/usr/local/bin/noder"]
