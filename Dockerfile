FROM node:22-bookworm-slim AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.25-bookworm AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY backend/ ./backend/
COPY cmd/ ./cmd/
COPY frontend/embed.go ./frontend/embed.go
COPY --from=frontend /src/frontend/dist ./frontend/dist
RUN CGO_ENABLED=1 go build -trimpath -o /jieli ./cmd/server

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates openssl \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 jieli \
    && mkdir -p /app/data && chown jieli:jieli /app/data
WORKDIR /app
COPY --from=server /jieli /usr/local/bin/jieli
COPY --chmod=755 scripts/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
ENV HOST=0.0.0.0 PORT=8080 JIELI_DEPLOYMENT_MODE=self-hosted DEV=0 \
    SQLITE_PATH=/app/data/jieli.db ATTACHMENT_STORAGE_DIR=/app/data/attachments GIN_MODE=release COOKIE_SECURE=false
USER jieli
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
