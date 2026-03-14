# syntax=docker/dockerfile:1.7

FROM golang:1.25-bookworm AS builder

WORKDIR /src

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/transaction-module ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata curl \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app

COPY --from=builder /out/transaction-module /usr/local/bin/transaction-module
COPY config /app/config
COPY migrations /app/migrations

ENV CONFIG_PATH=/app/config/config.yaml

EXPOSE 8080

USER app

ENTRYPOINT ["/usr/local/bin/transaction-module"]
