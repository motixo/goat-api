# Build
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache \
    ca-certificates \
    git

WORKDIR /src
COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/app \
    ./cmd/app

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/migrate \
    ./cmd/migrate

# Runtime image
FROM alpine:latest AS runtime

RUN apk add --no-cache \
        ca-certificates \
        wget \
    && adduser -D -u 10001 -s /sbin/nologin appuser

WORKDIR /app
COPY --from=builder --chown=appuser:appuser /out/app /app/app
COPY --from=builder --chown=appuser:appuser /out/migrate /app/migrate

USER 10001:10001

ENV ENV=production \
    GIN_MODE=release \
    SERVER_PORT=8080

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=150s --retries=3 \
    CMD wget --quiet --timeout=2 --tries=1 --spider "http://127.0.0.1:${SERVER_PORT}/api/ready" || exit 1

STOPSIGNAL SIGTERM

ENTRYPOINT ["/app/app"]
