# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app
RUN mkdir /app/build

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o build/machine-agent ./cmd/machine-agent

# Runtime stage
FROM debian:11-slim
WORKDIR /app
RUN addgroup --gid 65532 nonroot && \
    addgroup --gid 412 docker && \
    adduser --uid 65532 --gid 65532 --disabled-password --gecos "" nonroot && \
    adduser nonroot docker
RUN apt-get update && apt-get install -y docker.io && apt-get clean && rm -rf /var/lib/apt/lists/*    
COPY --from=builder /app/build/machine-agent /app/machine-agent
USER nonroot
ENTRYPOINT ["/app/machine-agent"]