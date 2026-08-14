#!/bin/bash
set -euo pipefail

# Hook baked into the metio mc-backup image (post-backup.sh) and enabled via
# POST_BACKUP_SCRIPT_FILE. The upstream itzg/mc-backup backup-loop invokes it
# after every backup with:
#   $1 = backup exit code
#   $2 = path to the backup tool's output log
#
# On success it atomically writes /manifests/latest.json describing the most
# recent backup. The machine-agent mounts the same directory at /manifests.
# A failed backup leaves the previous manifest in place so the dashboard keeps
# reporting the last known-good backup.

MANIFEST_DIR="${MANIFEST_DIR:-/manifests}"
BACKUP_STATUS="${1:-1}"
BACKUP_LOG="${2:-}"

if [ "${BACKUP_STATUS}" -ne 0 ]; then
  echo "mc-backup-manifest: backup failed (exit ${BACKUP_STATUS}); keeping previous manifest"
  exit 0
fi

# The restic backup log ends with a line like `snapshot 0123abcd saved`.
SNAPSHOT_ID=""
if [ -n "${BACKUP_LOG}" ] && [ -f "${BACKUP_LOG}" ]; then
  SNAPSHOT_ID="$(grep -Eo 'snapshot [a-f0-9]{8,} saved' "${BACKUP_LOG}" | tail -n1 | awk '{print $2}' || true)"
fi

# Fall back to querying the repository when the id was not in the log. Reuse
# the same tag filter the backup applies (BACKUP_NAME + RESTIC_ADDITIONAL_TAGS)
# so multiple worlds sharing one repository stay distinguishable.
TAG_FILTER="${BACKUP_NAME:-world}"
if [ -n "${RESTIC_ADDITIONAL_TAGS:-}" ]; then
  TAG_FILTER="${RESTIC_ADDITIONAL_TAGS},${TAG_FILTER}"
fi

if [ -z "${SNAPSHOT_ID}" ] && command -v restic >/dev/null 2>&1; then
  SNAPSHOT_ID="$(restic snapshots --json --latest 1 --tag "${TAG_FILTER}" 2>/dev/null | jq -r '.[0].id // empty' || true)"
fi

if [ -z "${SNAPSHOT_ID}" ]; then
  echo "mc-backup-manifest: could not resolve snapshot id; skipping manifest update"
  exit 0
fi

TIMESTAMP="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
SIZE_BYTES=0
if command -v restic >/dev/null 2>&1 && command -v jq >/dev/null 2>&1; then
  SNAPSHOT_JSON="$(restic snapshots --json --latest 1 --tag "${TAG_FILTER}" 2>/dev/null | jq '.[0] // empty' || true)"
  if [ -n "${SNAPSHOT_JSON}" ]; then
    TIMESTAMP="$(printf '%s' "${SNAPSHOT_JSON}" | jq -r '.time // empty' || true)"
    SIZE_BYTES="$(printf '%s' "${SNAPSHOT_JSON}" | jq -r '.summary.total_size // 0' || true)"
  fi
fi
[ -n "${TIMESTAMP}" ] || TIMESTAMP="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

mkdir -p "${MANIFEST_DIR}"
MANIFEST_FILE="${MANIFEST_DIR}/latest.json"
MANIFEST_TMP="${MANIFEST_FILE}.tmp"

jq -n \
  --arg timestamp "${TIMESTAMP}" \
  --arg snapshot_id "${SNAPSHOT_ID}" \
  --argjson size_bytes "${SIZE_BYTES}" \
  --arg method "${BACKUP_METHOD:-restic}" \
  '{timestamp: $timestamp, snapshot_id: $snapshot_id, size_bytes: $size_bytes, method: $method}' \
  > "${MANIFEST_TMP}"
mv "${MANIFEST_TMP}" "${MANIFEST_FILE}"

echo "mc-backup-manifest: wrote ${MANIFEST_FILE} (snapshot ${SNAPSHOT_ID})"