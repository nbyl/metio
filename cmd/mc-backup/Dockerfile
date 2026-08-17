# syntax=docker/dockerfile:1

# Wraps the upstream itzg/mc-backup image with a hook that publishes a small
# manifest (timestamp + restic snapshot id + byte size) after every successful
# backup. The machine-agent mounts the same directory at /manifests and will
# use the manifest to report when the last backup ran on the dashboard.
#
# The base image ships bash, jq and restic (Alpine), so the hook can parse the
# repository state directly.
FROM ghcr.io/itzg/mc-backup:latest

COPY cmd/mc-backup/post-backup.sh /post-backup.sh
USER root
RUN chmod +x /post-backup.sh

# Upstream contract: the backup-loop invokes POST_BACKUP_SCRIPT_FILE with
#   $1 = backup exit code
#   $2 = path to the backup tool's output log
ENV POST_BACKUP_SCRIPT_FILE=/post-backup.sh
