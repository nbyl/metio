#!/usr/bin/env bash
#
# SPIKE ONLY - ADR-0008 issue #547 (SP5).
#
# Verifies that a configuration change on the k8s path (Minecraft version,
# environment variable) is applied by updating the workload only, and that the
# VM is not recreated, restarted or otherwise disturbed. This is the central
# claim of ADR-0008: today every user-data change is ReplaceOnChanges, so a
# version or RCON change destroys the VM. On k8s it should be a pod swap.
#
# Methods:
#   run     - boot k3s + Minecraft on a steady (--no-spot) e2-medium VM, lock a
#             VM identity / world fingerprint, then apply two config changes:
#               A) Minecraft version 1.21.4 -> 1.21.3  (Deployment VERSION)
#               B) MAX_MEMORY 75% -> 70%               (Deployment env)
#             After each change: confirm new pod, verify world intact, verify
#             instance id / lastStartTimestamp / boot_id / uptime all unchanged,
#             and time apply -> mc-monitor accepting (the restart-only cost).
#   resume  - skip the bootstrap and run the baseline-lock + change phases
#             against an already-running harness cluster (e.g. after an aborted
#             run). Same evidence, no fresh VM.
#
# --no-spot is deliberate: a SPOT preemption would reset uptime and fake the
# "VM undisturbed" check. Preemption survival is #545's deliverable, not this
# ticket's. Results are appended to config-change-results.txt and must be
# recorded on #547; nothing here survives the spike.
#
# Requires: the spike.sh harness, docker (for mc-monitor), gcloud, sed.

set -u

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE="$HERE/spike.sh"
MANIFEST="$HERE/manifests/minecraft.yaml"
K3S_CONFIG="$HERE/configs/cos-k3s.yaml"
SCRATCH="$HERE/.config-change-scratch"

INSTANCE="spike-k3s"
ZONE="europe-west3-b"
RESULTS="$HERE/config-change-results.txt"

now() { date +%s; }
die() { echo "ERROR: $*" >&2; exit 1; }
info() { echo; echo "==> $*"; }

kubectl_rs() {
  # Same retry wrapper as test-preemption.sh: gcloud compute ssh can return
  # non-zero spuriously even on success, and a failing command must be visible.
  local tries=3
  while [ "$tries" -gt 0 ]; do
    # Quoting per-arg with %q is required here. The whole command becomes the
    # single --command string gcloud passes to the node shell, so without it
    # any embedded shell syntax (e.g. `sh -c "echo x > /data/world/..."`)
    # would be re-lexed at the NODE and fail - a redirect inside a container
    # must arrive intact as one argument to the container's shell.
    local q=() a
    for a in "$@"; do q+=("$(printf '%q' "$a")"); done
    if "$SPIKE" ssh --command "sudo KUBECONFIG=/etc/rancher/k3s/k3s.yaml kubectl ${q[*]}"; then
      return 0
    fi
    tries=$((tries - 1))
    [ "$tries" -gt 0 ] && sleep 3
  done
  return 1
}

ssh_rs() {
  local tries=3
  while [ "$tries" -gt 0 ]; do
    if "$SPIKE" ssh --command "$*"; then
      return 0
    fi
    tries=$((tries - 1))
    [ "$tries" -gt 0 ] && sleep 3
  done
  return 1
}

external_ip() {
  "$SPIKE" status 2>/dev/null | awk '{print $3}'
}

pod_name() {
  kubectl_rs get pod -l app=minecraft --no-headers | awk '{print $1; exit}'
}

deployment_env() {
  # Flat "NAME=VAL NAME=VAL ..." snapshot of the Deployment pod template env,
  # used to prove a changed field actually reached the running pod spec.
  # Rendered through yaml (not jsonpath) - jsonpath's {" "} range separator
  # survived the ssh hop unreliably and returned empty.
  local yaml_out
  yaml_out="$(kubectl_rs get deployment minecraft -o yaml 2>/dev/null)" || return 1
  printf '%s\n' "$yaml_out" | awk '
    /env:/                  {in_env=1; next}
    in_env && $2 == "name:" {name=$3}
    in_env && $1 == "value:" && name != "" {
      v=$2; gsub(/"/, "", v); print name "=" v; name=""
    }
    in_env && /resources:/  {exit}
  '
}

wait_instance_running() {
  local tries="${1:-60}"
  for _ in $(seq 1 "$tries"); do
    local st
    st="$(gcloud compute instances describe "$INSTANCE" --zone="$ZONE" --format='value(status)' 2>/dev/null || echo missing)"
    [ "$st" = "RUNNING" ] && return 0
    sleep 5
  done
  return 1
}

wait_node_ready() {
  local tries="${1:-90}"
  for _ in $(seq 1 "$tries"); do
    kubectl_rs get node --no-headers 2>/dev/null | awk '$2=="Ready"{found=1} END{exit !found}' && return 0
    sleep 10
  done
  return 1
}

wait_pod_ready() {
  local tries="${1:-40}"
  for _ in $(seq 1 "$tries"); do
    kubectl_rs get pod -l app=minecraft --no-headers 2>/dev/null | awk '$2=="1/1" && $3=="Running"{found=1} END{exit !found}' && return 0
    sleep 10
  done
  return 1
}

wait_world_ready() {
  local tries="${1:-60}"
  for _ in $(seq 1 "$tries"); do
    kubectl_rs exec "$(pod_name)" -- sh -c 'test -f /data/world/level.dat' 2>/dev/null && return 0
    sleep 10
  done
  return 1
}

wait_accepting() {
  local ip="$1" tries="${2:-80}"
  for _ in $(seq 1 "$tries"); do
    docker run --rm -q itzg/mc-monitor status --host "$ip" --port 25565 >/dev/null 2>&1 && return 0
    sleep 10
  done
  return 1
}

gcloud_describe_field() {
  gcloud compute instances describe "$INSTANCE" --zone="$ZONE" --format="value($1)" 2>/dev/null
}

# World fingerprint: region-file count + the .metio-config-marker contents.
world_fingerprint() {
  kubectl_rs exec "$(pod_name)" -- sh -c \
    'ls /data/world/region/*.mca 2>/dev/null | wc -l; cat /data/world/.metio-config-marker 2>/dev/null' 2>/dev/null
}

write_marker() {
  local tag="$1"
  kubectl_rs exec "$(pod_name)" -- sh -c "echo '$tag' > /data/world/.metio-config-marker && sync"
}

# VM identity lock. Any of these changing means the VM was disturbed:
#
#   id                 - stable for the VM's whole life; changes ONLY on recreate
#   lastStartTimestamp - changes on any (re)start
#   boot_id            - per-boot uuid in /proc; changes on any restart
#   uptime_seconds     - resets to ~0 on any restart; must stay monotonic
snapshot_identity() {
  INSTANCE_ID="$(gcloud_describe_field id)"
  LAST_START="$(gcloud_describe_field lastStartTimestamp)"
  BOOT_ID="$(ssh_rs 'cat /proc/sys/kernel/random/boot_id' 2>/dev/null)"
  UPTIME="$(ssh_rs 'cut -d" " -f1 /proc/uptime' 2>/dev/null)"
  TS="$(now)"
}

verify_identity() {
  local now_id now_start now_boot now_uptime
  now_id="$(gcloud_describe_field id)"
  now_start="$(gcloud_describe_field lastStartTimestamp)"
  now_boot="$(ssh_rs 'cat /proc/sys/kernel/random/boot_id' 2>/dev/null)"
  now_uptime="$(ssh_rs 'cut -d" " -f1 /proc/uptime' 2>/dev/null)"
  local id_ok=no start_ok=no boot_ok=no up_ok=no
  [ "$now_id" = "$INSTANCE_ID" ] && id_ok=yes
  [ "$now_start" = "$LAST_START" ] && start_ok=yes
  [ "$now_boot" = "$BOOT_ID" ] && boot_ok=yes
  # Downtime-free: current uptime >= baseline uptime + elapsed wall time (fudge).
  if awk -v u="$now_uptime" -v b="$UPTIME" -v s="$(( $(now) - TS ))" \
      'BEGIN{exit !(u >= b + s - 120)}' 2>/dev/null; then
    up_ok=yes
  fi
  printf 'instance_id=%s last_start=%s boot_id=%s uptime_monotonic=%s\n' \
    "$id_ok" "$start_ok" "$boot_ok" "$up_ok"
  [ "$id_ok" = "yes" ] && [ "$start_ok" = "yes" ] && [ "$boot_ok" = "yes" ] && [ "$up_ok" = "yes" ]
}

# --- phases -------------------------------------------------------------------

phase_bootstrap() {
  info "=== phase 0: create steady (--no-spot) VM and bootstrap k3s ==="
  "$SPIKE" create --os cos --user-data "$K3S_CONFIG" --no-spot
  local ip; ip="$(external_ip)"
  info "external IP: $ip"

  wait_instance_running || die "instance never reached RUNNING"
  local t0; t0="$(now)"
  wait_node_ready || die "k3s node never Ready"
  cat "$MANIFEST" | "$SPIKE" ssh --command 'sudo KUBECONFIG=/etc/rancher/k3s/k3s.yaml kubectl apply -f -' \
    || die "initial apply failed"
  wait_pod_ready || die "minecraft pod never Ready on first boot"
  wait_world_ready || die "world never created on first boot"
  wait_accepting "$ip" 80 || die "minecraft never accepting on first boot"
  local first_boot=$(( $(now) - t0 ))
  printf 'first_boot_accept_seconds=%s\n' "$first_boot" >> "$RESULTS"
  info "first boot: ${first_boot}s to accepting"
}

phase_changes() {
  local ip; ip="$(external_ip)"
  [ -n "$ip" ] || die "cannot determine external IP - is the harness cluster running?"

  info "=== phase 1: lock VM identity and world fingerprint ==="
  local tag base_fp old_env
  tag="config-baseline-$(date +%s)"
  info "writing integrity marker $tag"
  write_marker "$tag" || die "baseline marker write failed"
  base_fp="$(world_fingerprint)"
  old_env="$(deployment_env)"
  info "deployment env before changes: $old_env"
  snapshot_identity
  info "identity: id=$INSTANCE_ID start=$LAST_START boot=$BOOT_ID uptime=$UPTIME"

  {
    printf 'instance_id=%s\n' "$INSTANCE_ID"
    printf 'last_start=%s\n' "$LAST_START"
    printf 'boot_id=%s\n' "$BOOT_ID"
    printf 'baseline_uptime=%s\n' "$UPTIME"
    printf 'baseline_marker=%s\n' "$tag"
    printf 'baseline_region_files=%s\n' "$(printf '%s' "$base_fp" | head -1)"
    printf 'baseline_env=%s\n' "$old_env"
  } >> "$RESULTS"

  # --- change A: Minecraft version ------------------------------------------
  info "=== phase 2: change A - Minecraft version 1.21.4 -> 1.21.3 ==="
  sed 's/value: "1.21.4"/value: "1.21.3"/' "$MANIFEST" > "$SCRATCH"
  local old_pod t_a t_b new_pod new_env fp_a id_a ver_log
  old_pod="$(pod_name)"

  info "applying (pod ${old_pod}) ..."
  t_a="$(now)"
  cat "$SCRATCH" | "$SPIKE" ssh --command 'sudo KUBECONFIG=/etc/rancher/k3s/k3s.yaml kubectl apply -f -' \
    || die "apply of version change failed"
  wait_pod_ready || die "new minecraft pod never Ready after version change"
  wait_world_ready || die "world not ready after version change"
  wait_accepting "$ip" 80 || die "minecraft never accepting after version change"
  t_b="$(now)"

  new_pod="$(pod_name)"
  new_env="$(deployment_env)"
  fp_a="$(world_fingerprint)"
  id_a="$(verify_identity)"
  ver_log="$(kubectl_rs logs "$new_pod" 2>/dev/null | grep -m1 'Starting minecraft server version' || echo UNKNOWN)"

  {
    printf 'change_a_apply_to_accept_seconds=%s\n' "$((t_b - t_a))"
    printf 'change_a_pod_swapped=%s\n' "$([ "$old_pod" != "$new_pod" ] && echo yes || echo no)"
    printf 'change_a_pod=%s\n' "$new_pod"
    printf 'change_a_env=%s\n' "$new_env"
    printf 'change_a_server_log=%s\n' "$ver_log"
    printf 'change_a_region_files=%s\n' "$(printf '%s' "$fp_a" | head -1)"
    printf 'change_a_marker=%s\n' "$(printf '%s' "$fp_a" | sed -n 2p)"
    printf 'change_a_identity=%s\n' "$id_a"
  } >> "$RESULTS"
  info "version change: pod $( [ "$old_pod" != "$new_pod" ] && echo swapped || echo UNCHANGED ); apply -> accepting $((t_b - t_a))s"
  info "world: $(printf '%s' "$fp_a" | head -1) region files, marker $(printf '%s' "$fp_a" | sed -n 2p); identity: $id_a"

  # --- change B: MAX_MEMORY ---------------------------------------------------
  info "=== phase 3: change B - MAX_MEMORY 75% -> 70% ==="
  sed -e 's/value: "1.21.4"/value: "1.21.3"/' -e 's/value: "75%"/value: "70%"/' "$MANIFEST" > "$SCRATCH"
  old_pod="$(pod_name)"

  info "applying (pod ${old_pod}) ..."
  t_a="$(now)"
  cat "$SCRATCH" | "$SPIKE" ssh --command 'sudo KUBECONFIG=/etc/rancher/k3s/k3s.yaml kubectl apply -f -' \
    || die "apply of env change failed"
  wait_pod_ready || die "new minecraft pod never Ready after env change"
  wait_world_ready || die "world not ready after env change"
  wait_accepting "$ip" 80 || die "minecraft never accepting after env change"
  t_b="$(now)"

  new_pod="$(pod_name)"
  new_env="$(deployment_env)"
  local fp_b; fp_b="$(world_fingerprint)"
  local id_b; id_b="$(verify_identity)"
  local ver_log_b
  ver_log_b="$(kubectl_rs logs "$new_pod" 2>/dev/null | grep -m1 'Starting minecraft server version' || echo UNKNOWN)"

  {
    printf 'change_b_apply_to_accept_seconds=%s\n' "$((t_b - t_a))"
    printf 'change_b_pod_swapped=%s\n' "$([ "$old_pod" != "$new_pod" ] && echo yes || echo no)"
    printf 'change_b_pod=%s\n' "$new_pod"
    printf 'change_b_env=%s\n' "$new_env"
    printf 'change_b_server_log=%s\n' "$ver_log_b"
    printf 'change_b_region_files=%s\n' "$(printf '%s' "$fp_b" | head -1)"
    printf 'change_b_marker=%s\n' "$(printf '%s' "$fp_b" | sed -n 2p)"
    printf 'change_b_identity=%s\n' "$id_b"
  } >> "$RESULTS"

  info "env change: pod $( [ "$old_pod" != "$new_pod" ] && echo swapped || echo UNCHANGED ); apply -> accepting $((t_b - t_a))s"
  info "world: $(printf '%s' "$fp_b" | head -1) region files, marker $(printf '%s' "$fp_b" | sed -n 2p); identity: $id_b"
}

phase_summary() {
  info "=== phase 4: summary ==="
  echo
  cat "$RESULTS"
}

# --- commands -----------------------------------------------------------------

cmd_run() {
  rm -f "$SCRATCH"
  : > "$RESULTS"
  phase_bootstrap
  phase_changes
  phase_summary
  "$SPIKE" destroy >/dev/null
  echo; echo "Teardown complete. Verify with: $SPIKE list"
}

cmd_resume() {
  # Reuse an already-running harness cluster (e.g. after an aborted `run`).
  # Records pod-level first-boot context from the running pod before changing.
  rm -f "$SCRATCH"
  : > "$RESULTS"
  info "resuming against existing cluster on $("$SPIKE" status 2>/dev/null)"
  local pod f; pod="$(pod_name)"
  f="$(kubectl_rs get pod "$pod" -o yaml 2>/dev/null)"
  if [ -n "$f" ]; then
    local created ready ms
    created="$(printf '%s' "$f" | awk '/creationTimestamp:/{print $2; exit}')"
    ready="$(printf '%s' "$f" | awk '/lastTransitionTime:/{t=$2} /type: Ready/{print t; exit}')"
    ms="UNKNOWN"
    created="$(date -d "$created" +%s 2>/dev/null)"
    ready="$(date -d "$ready" +%s 2>/dev/null)"
    [ -n "$created" ] && [ -n "$ready" ] && ms=$((ready - created))
    printf 'resume_first_boot_pod_seconds=%s\n' "$ms" >> "$RESULTS"
    info "existing pod ${pod} Ready in ${ms}s (pod-level cold start)"
  fi
  phase_changes
  phase_summary
  "$SPIKE" destroy >/dev/null
  echo; echo "Teardown complete. Verify with: $SPIKE list"
}

usage() {
  cat <<'EOF'
SPIKE ONLY - config-change-avoids-VM-replacement test for #547.

Usage: test-config-change.sh run|resume

  run     Create VM, bootstrap k3s+Minecraft, then run the change phases.
  resume  Skip bootstrap; run the change phases against an existing cluster.

Results are written to config-change-results.txt.
EOF
}

main() {
  local cmd="${1:-}"
  case "$cmd" in
    run)    cmd_run ;;
    resume) cmd_resume ;;
    *)      usage; exit 1 ;;
  esac
}

main "$@"