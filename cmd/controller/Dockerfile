# syntax=docker/dockerfile:1

# Stage 1: Frontend build
FROM node:22-alpine AS frontend-builder
WORKDIR /app
COPY frontend/package*.json ./frontend/
RUN cd frontend && npm ci
COPY frontend/ ./frontend/
RUN cd frontend && npm run build

# Stage 2: Go build
FROM golang:1.25-alpine AS builder
WORKDIR /app
RUN mkdir /app/build

# Copy built frontend assets from frontend stage
COPY --from=frontend-builder /app/static/dist ./static/dist
COPY static/embed.go ./static/

# Build Go binary
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=local
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.Version=${VERSION}" -o build/controller ./cmd/controller

# Stage 3: Runtime
FROM gcr.io/distroless/static-debian11
EXPOSE 8080
WORKDIR /app
COPY --from=builder /app/build/controller /app/controller
USER nonroot:nonroot
ENTRYPOINT ["/app/controller"]
