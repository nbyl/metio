# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.26-alpine AS builder
WORKDIR /app
RUN mkdir /app/build

RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
ARG VERSION=local
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags "-X main.Version=${VERSION}" -o build/machine-agent ./cmd/machine-agent

# Runtime stage
FROM debian:13-slim
WORKDIR /app
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      adduser \
      ca-certificates \
      docker-cli \
      util-linux && \
    addgroup --gid 65532 nonroot && \
    addgroup --gid 412 docker && \
    adduser --uid 65532 --gid 65532 --disabled-password --gecos "" nonroot && \
    adduser nonroot docker && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/build/machine-agent /app/machine-agent
USER nonroot
ENTRYPOINT ["/app/machine-agent"]
