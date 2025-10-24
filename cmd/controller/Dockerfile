# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app
RUN mkdir /app/build

RUN apk add --no-cache nodejs npm
COPY package*.json ./
RUN npm ci

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go generate ./...
RUN CGO_ENABLED=0 GOOS=linux go build -o build/controller ./cmd/controller

# Runtime stage
FROM gcr.io/distroless/static-debian11
EXPOSE 8080
WORKDIR /app
COPY --from=builder /app/build/controller /app/controller
COPY static/ /app/static/
USER nonroot:nonroot
ENTRYPOINT ["/app/controller"]
