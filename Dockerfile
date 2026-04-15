FROM --platform=linux/amd64 node:22-alpine AS frontend-builder

WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --ignore-scripts
COPY frontend/ ./
ENV NODE_OPTIONS=--max-old-space-size=4096
ENV VITE_WEB=true
RUN npm run build

FROM golang:1.25-alpine AS backend-builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
ARG APP_VERSION=0.0.0
RUN CGO_ENABLED=0 GOOS=linux GOFLAGS="-mod=mod" go build -tags web -ldflags "-s -w -X main.version=${APP_VERSION}" -o /app/yt-go-server .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata && rm -rf /var/cache/apk/* /tmp/*

WORKDIR /app
COPY --from=backend-builder /app/yt-go-server ./
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

EXPOSE 8080

ENV YTGO_WEB_ADDR=:8080
ENV XDG_CONFIG_HOME=/app

CMD ["./yt-go-server"]