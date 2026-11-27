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
RUN apk add --no-cache upx ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY web/embed.go ./web/embed.go
COPY --from=web-builder /src/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X main.version=${APP_VERSION}" \
    -o /out/rclone-manager ./cmd/rclone-manager \
 && upx --best -q /out/rclone-manager \
 && mkdir -p /rootfs/config/logs /rootfs/config/.cache /rootfs/tmp /rootfs/etc/ssl/certs \
 && cp /etc/ssl/certs/ca-certificates.crt /rootfs/etc/ssl/certs/ca-certificates.crt \
 && printf 'app:x:1000:1000:app:/config:/sbin/nologin\n' > /rootfs/etc/passwd \
 && printf 'app:x:1000:\n' > /rootfs/etc/group \
 && chown -R 1000:1000 /rootfs/config \
 && chmod 1777 /rootfs/tmp

FROM scratch
ARG RCLONE_VERSION=1.76.0
ARG APP_VERSION=dev
LABEL org.opencontainers.image.title="rclone-manager" \
      org.opencontainers.image.description="Self-hosted, provider-agnostic rclone backup & cloud storage manager" \
      org.opencontainers.image.source="https://github.com/kevindiu/rclone-manager" \
      org.opencontainers.image.version="${APP_VERSION}" \
      org.rclone.version="${RCLONE_VERSION}"

COPY --from=go-builder /rootfs/ /
COPY --from=rclone-bin /usr/local/bin/rclone /usr/local/bin/rclone
COPY --from=go-builder /out/rclone-manager /usr/local/bin/rclone-manager

USER 1000:1000
VOLUME ["/config"]

ENV RCLONE_MANAGER_DATA_DIR=/config \
    PORT=8080 \
    HOME=/config \
    XDG_CACHE_HOME=/config/.cache \
    TMPDIR=/tmp

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/rclone-manager", "healthcheck"]

ENTRYPOINT ["/usr/local/bin/rclone-manager"]
