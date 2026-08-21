# syntax=docker/dockerfile:1

# Stage 1: Frontend build
FROM node:26-alpine AS frontend-builder
WORKDIR /app
COPY web/package*.json ./web/
RUN cd web && npm ci
COPY web/ ./web/
RUN cd web && npm run build

# Stage 2: Go build
FROM golang:1.26-alpine AS builder
WORKDIR /app
RUN mkdir /app/build

# Install Pulumi CLI
ARG PULUMI_VERSION=3.231.0
RUN apk add --no-cache curl && \
    curl -fsSL "https://github.com/pulumi/pulumi/releases/download/v${PULUMI_VERSION}/pulumi-v${PULUMI_VERSION}-linux-x64.tar.gz" | \
    tar -xz -C /usr/local/bin --strip-components=1 pulumi/pulumi

# Download Go dependencies before the frontend COPY so web changes do not
# invalidate the module layers
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy built frontend assets from frontend stage
COPY --from=frontend-builder /app/static/dist ./static/dist
COPY static/embed.go ./static/

# Build Go binary
COPY . .
ARG VERSION=local
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.Version=${VERSION}" -o build/controller ./cmd/controller

# Stage 3: Runtime
FROM alpine:3.24
RUN apk add --no-cache ca-certificates && \
    adduser -D -u 65532 nonroot
EXPOSE 8080
WORKDIR /app
COPY --from=builder /app/build/controller /app/controller
COPY --from=builder /usr/local/bin/pulumi /usr/local/bin/pulumi
ENV PULUMI_HOME=/tmp/.pulumi
USER nonroot:nonroot
ENTRYPOINT ["/app/controller"]
