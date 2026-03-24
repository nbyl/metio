# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app
RUN mkdir /app/build

RUN apk add --no-cache nodejs npm git

# Install frontend dependencies and build
COPY frontend/package*.json ./frontend/
RUN cd frontend && npm ci
COPY frontend/ ./frontend/
RUN cd frontend && npm run build

# Build Go binary (frontend is embedded via go:embed)
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=local
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.Version=${VERSION}" -o build/controller ./cmd/controller

# Runtime stage
FROM gcr.io/distroless/static-debian11
EXPOSE 8080
WORKDIR /app
COPY --from=builder /app/build/controller /app/controller
USER nonroot:nonroot
ENTRYPOINT ["/app/controller"]
