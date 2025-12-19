# Build
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache \
    ca-certificates \
    git \


WORKDIR /app
COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go install github.com/google/wire/cmd/wire@latest
RUN wire ./cmd/app

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o build/bin/app ./cmd/app


# Runtime image
FROM alpine:latest AS runtime

RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /app/build/bin/app /app/app
COPY .env .env

RUN adduser -D -s /bin/sh appuser
RUN chown -R appuser:appuser /app
USER appuser

EXPOSE 8080

CMD ["./app"]