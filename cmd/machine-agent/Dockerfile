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
FROM gcr.io/distroless/static-debian11
WORKDIR /app
COPY --from=builder /app/build/machine-agent /app/machine-agent
USER nonroot:nonroot
ENTRYPOINT ["/app/machine-agent"]