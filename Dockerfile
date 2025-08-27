# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app
RUN mkdir /app/build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o build/server ./cmd/main.go

# Runtime stage
FROM gcr.io/distroless/static-debian11
EXPOSE 8080
WORKDIR /app
COPY --from=builder /app/build/server /app/server
COPY static/ /app/static/
USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
