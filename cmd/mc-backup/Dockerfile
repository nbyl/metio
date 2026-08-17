# syntax=docker/dockerfile:1

# Wraps the upstream itzg/mc-backup image with a hook that publishes a small
# manifest (timestamp + restic snapshot id + byte size) after every successful
# backup. The machine-agent mounts the same directory at /manifests and will
# use the manifest to report when the last backup ran on the dashboard.
#
# The hook is a statically-linked Go binary (cmd/mc-backup/post-backup) that
# shells out to the restic binary shipped in the base image.
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/mc-backup/post-backup ./cmd/mc-backup/post-backup
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /post-backup ./cmd/mc-backup/post-backup

FROM ghcr.io/itzg/mc-backup:latest

COPY --from=builder /post-backup /usr/local/bin/post-backup

# Upstream contract: the backup-loop invokes POST_BACKUP_SCRIPT_FILE with
#   $1 = backup exit code
#   $2 = path to the backup tool's output log
ENV POST_BACKUP_SCRIPT_FILE=/usr/local/bin/post-backup