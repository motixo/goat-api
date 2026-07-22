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

# Runtime image
FROM alpine:latest AS runtime

RUN apk add --no-cache ca-certificates \
    && adduser -D -s /sbin/nologin appuser

WORKDIR /app
COPY --from=builder --chown=appuser:appuser /out/app /app/app

USER appuser

ENV ENV=production \
    GIN_MODE=release \
    SERVER_PORT=8080

EXPOSE 8080

ENTRYPOINT ["/app/app"]
