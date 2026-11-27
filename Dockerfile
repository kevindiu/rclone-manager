# Pinned rclone version (never floating :latest) so upgrades are explicit & reproducible.
ARG RCLONE_VERSION=1.76.0
ARG APP_VERSION=dev

FROM rclone/rclone:${RCLONE_VERSION} AS rclone-bin

FROM --platform=$BUILDPLATFORM node:26-alpine AS web-builder
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS go-builder
ARG TARGETOS
ARG TARGETARCH
ARG APP_VERSION
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY web/embed.go ./web/embed.go
COPY --from=web-builder /src/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X main.version=${APP_VERSION}" \
    -o /out/rclone-manager ./cmd/rclone-manager

FROM alpine:3.24
ARG RCLONE_VERSION=1.76.0
ARG APP_VERSION=dev
LABEL org.opencontainers.image.title="rclone-manager" \
      org.opencontainers.image.description="Self-hosted, provider-agnostic rclone backup & cloud storage manager" \
      org.opencontainers.image.source="https://github.com/kevindiu/rclone-manager" \
      org.opencontainers.image.version="${APP_VERSION}" \
      org.rclone.version="${RCLONE_VERSION}"

RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -g 1000 app \
 && adduser -D -u 1000 -G app app \
 && mkdir -p /config /config/logs \
 && chown -R app:app /config

COPY --from=rclone-bin /usr/local/bin/rclone /usr/local/bin/rclone
COPY --from=go-builder /out/rclone-manager /usr/local/bin/rclone-manager

USER app
VOLUME ["/config"]

ENV RCLONE_MANAGER_DATA_DIR=/config \
    PORT=8080

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO- "http://127.0.0.1:${PORT:-8080}/api/health" >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/usr/local/bin/rclone-manager"]
