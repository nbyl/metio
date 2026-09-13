#!/usr/bin/env bash
#
# SPIKE ONLY - delete at the end of milestone #10 (ADR-0008).
#
# Throwaway harness for the ADR-0008 Kubernetes feasibility spike (#542).
# Stands up and tears down a single GCE VM that mirrors the shape of a real
# Metio server, so each k3s experiment starts from a known-clean machine.
#
# This deliberately does NOT touch internal/pulumi/. See tools/spike-k3s/README.md.

set -euo pipefail

PREFIX="spike-k3s"
LABEL="purpose=adr0008-spike"
TAG="spike-k3s"

# Defaults chosen to be representative of a real server:
#   e2-medium      - 2 vCPU / 4 GB, present in db.MachineTypes
#   europe-west3-b - matches the Makefile's LOCATION default
#   20 GB          - matches the setup wizard's INITIAL_FORM diskSizeGB
OS="cos"
MACHINE_TYPE="e2-medium"
ZONE="europe-west3-b"
DISK_SIZE="20"
USER_DATA=""
SPOT="yes"

INSTANCE="${PREFIX}"
DISK="${PREFIX}-data"
ADDRESS="${PREFIX}-addr"
FIREWALL="${PREFIX}-fw"

# Image identifiers, all verified against the live API on 2026-09-11.
#
# Flatcar note: images in kinvolk-public can be USED but not LISTED by an
# ordinary account. `gcloud compute images list --project kinvolk-public`
# fails with a permissions error while describe-from-family succeeds. Never
# validate these by listing; reference the family directly.
image_project_for() {
  case "$1" in
    cos)     echo "cos-cloud" ;;
    flatcar) echo "kinvolk-public" ;;
    ubuntu)  echo "ubuntu-os-cloud" ;;
    *)       die "unknown --os '$1' (expected: cos, flatcar, ubuntu)" ;;
  esac
}

image_family_for() {
  case "$1" in
    cos)     echo "cos-stable" ;;
    flatcar) echo "flatcar-stable" ;;
    ubuntu)  echo "ubuntu-2404-lts-amd64" ;;
    *)       die "unknown --os '$1' (expected: cos, flatcar, ubuntu)" ;;
  esac
}

die()  { echo "ERROR: $*" >&2; exit 1; }
info() { echo "==> $*"; }

require_gcloud() {
  command -v gcloud >/dev/null 2>&1 || die "gcloud is not installed"
  local account
  account="$(gcloud auth list --filter='status:ACTIVE' --format='value(account)' 2>/dev/null | head -n1)"
  [ -n "$account" ] || die "no active gcloud account; run: gcloud auth login"
  PROJECT="$(gcloud config get-value project 2>/dev/null)"
  [ -n "$PROJECT" ] && [ "$PROJECT" != "(unset)" ] || die "no gcloud project set; run: gcloud config set project <id>"
  REGION="${ZONE%-*}"
}

usage() {
  cat <<'EOF'
SPIKE ONLY - throwaway VM harness for the ADR-0008 k3s spike (#542).

Usage:
  spike.sh create [flags]   Create the VM and its supporting resources
  spike.sh destroy          Delete everything this script creates
  spike.sh list             Show what currently exists (leak check)
  spike.sh ssh [args...]    SSH into the VM
  spike.sh status           Print instance name, IP and boot disk image

Flags for create:
  --os cos|flatcar|ubuntu   OS image        (default: cos)
  --machine-type TYPE       Machine type    (default: e2-medium)
  --zone ZONE               Zone            (default: europe-west3-b)
  --disk-size GB            Data disk size  (default: 20)
  --user-data FILE          Provisioning config passed as the user-data key
                            (cloud-init for cos/ubuntu, Ignition for flatcar)
  --no-spot                 Create an on-demand VM instead of SPOT.
                            DEBUGGING ONLY - SP4 (#545) requires real SPOT
                            preemption semantics and must not use this.

Examples:
  ./spike.sh create --os cos
  ./spike.sh create --os flatcar --user-data ignition.json
  ./spike.sh create --os ubuntu --machine-type e2-small   # for SP5 (#546)
  ./spike.sh ssh
  ./spike.sh destroy
EOF
}

parse_create_flags() {
  while [ $# -gt 0 ]; do
    case "$1" in
      --os)           OS="${2:?--os needs a value}"; shift 2 ;;
      --machine-type) MACHINE_TYPE="${2:?--machine-type needs a value}"; shift 2 ;;
      --zone)         ZONE="${2:?--zone needs a value}"; shift 2 ;;
      --disk-size)    DISK_SIZE="${2:?--disk-size needs a value}"; shift 2 ;;
      --user-data)    USER_DATA="${2:?--user-data needs a value}"; shift 2 ;;
      --no-spot)      SPOT="no"; shift ;;
      -h|--help)      usage; exit 0 ;;
      *)              die "unknown flag '$1' (try --help)" ;;
    esac
  done
}

cmd_create() {
  parse_create_flags "$@"
  require_gcloud

  local image_project image_family
  image_project="$(image_project_for "$OS")"
  image_family="$(image_family_for "$OS")"

  if [ -n "$USER_DATA" ]; then
    [ -f "$USER_DATA" ] || die "--user-data file not found: $USER_DATA"
  fi

  info "project=$PROJECT zone=$ZONE os=$OS ($image_project/$image_family)"
  info "machine-type=$MACHINE_TYPE disk-size=${DISK_SIZE}GB spot=$SPOT"

  # --- static address, mirroring internal/pulumi/programs/server.go:348 ---
  if gcloud compute addresses describe "$ADDRESS" --region="$REGION" >/dev/null 2>&1; then
    info "address $ADDRESS already exists"
  else
    info "creating address $ADDRESS"
    gcloud compute addresses create "$ADDRESS" --region="$REGION"
  fi
  local ip
  ip="$(gcloud compute addresses describe "$ADDRESS" --region="$REGION" --format='value(address)')"

  # --- firewall, mirroring server.go:355 (icmp + tcp:25565) ---
  if gcloud compute firewall-rules describe "$FIREWALL" >/dev/null 2>&1; then
    info "firewall $FIREWALL already exists"
  else
    info "creating firewall $FIREWALL"
    gcloud compute firewall-rules create "$FIREWALL" \
      --network=default \
      --allow=icmp,tcp:25565 \
      --source-ranges=0.0.0.0/0 \
      --target-tags="$TAG"
  fi

  # --- data disk, mirroring server.go:327 (pd-standard) ---
  # Attached RAW on purpose. Formatting and mounting differ per OS
  # (cloud-init fs_setup vs Ignition) and belong to SP2 (#543).
  if gcloud compute disks describe "$DISK" --zone="$ZONE" >/dev/null 2>&1; then
    info "disk $DISK already exists"
  else
    info "creating disk $DISK (${DISK_SIZE}GB pd-standard)"
    gcloud compute disks create "$DISK" \
      --zone="$ZONE" \
      --type=pd-standard \
      --size="${DISK_SIZE}GB" \
      --labels="$LABEL"
  fi

  # --- instance, mirroring server.go:380 + scheduling at :407-409 ---
  if gcloud compute instances describe "$INSTANCE" --zone="$ZONE" >/dev/null 2>&1; then
    die "instance $INSTANCE already exists; run './spike.sh destroy' first"
  fi

  local -a args=(
    "$INSTANCE"
    --zone="$ZONE"
    --machine-type="$MACHINE_TYPE"
    --image-project="$image_project"
    --image-family="$image_family"
    --address="$ip"
    --tags="$TAG"
    --labels="$LABEL,spike_os=$OS"
    # device-name matches server_cloud_config.yml:2, so the disk appears at
    # /dev/disk/by-id/google-minecraft-data exactly as in production.
    --disk="name=$DISK,device-name=minecraft-data,mode=rw,auto-delete=no"
  )

  if [ "$SPOT" = "yes" ]; then
    args+=(--provisioning-model=SPOT --no-restart-on-failure --maintenance-policy=TERMINATE)
  else
    info "WARNING: --no-spot set; this VM does NOT mirror production scheduling"
  fi

  if [ -n "$USER_DATA" ]; then
    args+=(--metadata-from-file="user-data=$USER_DATA")
  fi

  info "creating instance $INSTANCE"
  if ! gcloud compute instances create "${args[@]}"; then
    die "instance creation failed. If this is a SPOT capacity error, retry, pick another --zone, or use --no-spot for non-preemption work."
  fi

  echo
  info "ready: $INSTANCE at $ip"
  info "ssh:   $0 ssh"
  info "clean: $0 destroy"
}

cmd_destroy() {
  require_gcloud
  info "project=$PROJECT zone=$ZONE"

  # Dependency order: instance, then disk, then address, then firewall.
  if gcloud compute instances describe "$INSTANCE" --zone="$ZONE" >/dev/null 2>&1; then
    info "deleting instance $INSTANCE"
    gcloud compute instances delete "$INSTANCE" --zone="$ZONE" --quiet
  else
    info "instance $INSTANCE not present"
  fi

  if gcloud compute disks describe "$DISK" --zone="$ZONE" >/dev/null 2>&1; then
    info "deleting disk $DISK"
    gcloud compute disks delete "$DISK" --zone="$ZONE" --quiet
  else
    info "disk $DISK not present"
  fi

  if gcloud compute addresses describe "$ADDRESS" --region="$REGION" >/dev/null 2>&1; then
    info "deleting address $ADDRESS"
    gcloud compute addresses delete "$ADDRESS" --region="$REGION" --quiet
  else
    info "address $ADDRESS not present"
  fi

  if gcloud compute firewall-rules describe "$FIREWALL" >/dev/null 2>&1; then
    info "deleting firewall $FIREWALL"
    gcloud compute firewall-rules delete "$FIREWALL" --quiet
  else
    info "firewall $FIREWALL not present"
  fi

  echo
  info "destroy complete; verify with: $0 list"
}

cmd_list() {
  require_gcloud
  echo "project: $PROJECT"
  echo
  echo "instances:"
  gcloud compute instances list --filter="name~^${PREFIX}" \
    --format='table(name,zone,machineType.basename(),status,scheduling.provisioningModel)' 2>/dev/null || true
  echo
  echo "disks:"
  gcloud compute disks list --filter="name~^${PREFIX}" \
    --format='table(name,zone,sizeGb,type.basename())' 2>/dev/null || true
  echo
  echo "addresses:"
  gcloud compute addresses list --filter="name~^${PREFIX}" \
    --format='table(name,region,address,status)' 2>/dev/null || true
  echo
  echo "firewall rules:"
  gcloud compute firewall-rules list --filter="name~^${PREFIX}" \
    --format='table(name,network,allowed[].map().firewall_rule().list())' 2>/dev/null || true
}

cmd_ssh() {
  require_gcloud
  exec gcloud compute ssh "$INSTANCE" --zone="$ZONE" "$@"
}

cmd_status() {
  require_gcloud
  gcloud compute instances describe "$INSTANCE" --zone="$ZONE" \
    --format='value(name,status,networkInterfaces[0].accessConfigs[0].natIP,disks[0].licenses[0].basename(),scheduling.provisioningModel)'
}

main() {
  local cmd="${1:-}"
  [ $# -gt 0 ] && shift || true
  case "$cmd" in
    create)  cmd_create "$@" ;;
    destroy) cmd_destroy "$@" ;;
    list)    cmd_list "$@" ;;
    ssh)     cmd_ssh "$@" ;;
    status)  cmd_status "$@" ;;
    -h|--help|help|"") usage ;;
    *)       die "unknown command '$cmd' (try --help)" ;;
  esac
}

main "$@"
