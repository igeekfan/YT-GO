ARG NODE_IMAGE=node:22-alpine
ARG GO_IMAGE=golang:1.25-alpine
ARG RUNTIME_IMAGE=debian:bookworm-slim

FROM ${NODE_IMAGE} AS frontend-builder

WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --ignore-scripts
COPY frontend/ ./
ENV NODE_OPTIONS=--max-old-space-size=4096
ENV VITE_WEB=true
RUN npm run build

FROM ${GO_IMAGE} AS backend-builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY internal/ ./internal/
COPY main_web.go version.go wails.json ./
ARG APP_VERSION=0.0.0
RUN CGO_ENABLED=0 GOOS=linux GOFLAGS="-mod=mod" go build -tags web -ldflags "-s -w -X main.version=${APP_VERSION}" -o /app/yt-go-server .

FROM ${RUNTIME_IMAGE} AS runtime-tools

ARG TARGETARCH
ARG DENO_VERSION=latest
ARG YTDLP_VERSION=latest

RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get -o Acquire::Retries=5 update \
    && apt-get -o Acquire::Retries=5 install -y --no-install-recommends ca-certificates curl unzip

RUN set -eux; \
    case "${TARGETARCH}" in \
        amd64) deno_arch="x86_64"; ytdlp_asset="yt-dlp_linux" ;; \
        arm64) deno_arch="aarch64"; ytdlp_asset="yt-dlp_linux_aarch64" ;; \
        *) echo "unsupported architecture: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    if [ "${DENO_VERSION}" = "latest" ]; then \
        deno_release="latest/download"; \
    else \
        deno_release="download/${DENO_VERSION}"; \
    fi; \
    if [ "${YTDLP_VERSION}" = "latest" ]; then \
        ytdlp_release="latest/download"; \
    else \
        ytdlp_release="download/${YTDLP_VERSION}"; \
    fi; \
    curl -fsSL "https://github.com/denoland/deno/releases/${deno_release}/deno-${deno_arch}-unknown-linux-gnu.zip" -o /tmp/deno.zip; \
    unzip /tmp/deno.zip -d /usr/local/bin; \
    curl -fsSL "https://github.com/yt-dlp/yt-dlp/releases/${ytdlp_release}/${ytdlp_asset}" -o /usr/local/bin/yt-dlp; \
    chmod 0755 /usr/local/bin/deno /usr/local/bin/yt-dlp; \
    deno --version; \
    yt-dlp --version

FROM ${RUNTIME_IMAGE}

RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get -o Acquire::Retries=5 update \
    && DEBIAN_FRONTEND=noninteractive apt-get -o Acquire::Retries=5 install -y --no-install-recommends ca-certificates curl ffmpeg tzdata

WORKDIR /app
COPY --from=backend-builder /app/yt-go-server ./
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist
COPY --from=runtime-tools /usr/local/bin/deno /usr/local/bin/deno
COPY --from=runtime-tools /usr/local/bin/yt-dlp /usr/local/bin/yt-dlp

RUN mkdir -p /data/config /data/downloads

EXPOSE 8080

ENV YTGO_WEB_ADDR=:8080 \
    YTGO_DOWNLOAD_DIR=/data/downloads \
    YTGO_YTDLP_PATH=/usr/local/bin/yt-dlp \
    XDG_CONFIG_HOME=/data/config \
    XDG_CACHE_HOME=/data/config/cache \
    DENO_DIR=/data/config/deno

VOLUME ["/data/config", "/data/downloads"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl --fail --silent --show-error http://127.0.0.1:8080/api/health || exit 1

CMD ["./yt-go-server"]
