#!/usr/bin/env bash
#
# SPIKE ONLY - ADR-0008 issue #545 (SP4).
#
# Verifies that a single-node k3s cluster and its Minecraft workload survive
# hard preemption (machine destroyed without warning).
#
# Methods:
#   baseline   - measure cold boot of the CURRENT production runtime
#                (systemd + docker) so k3s recovery can be compared to it.
#   k8s        - boot k3s + Minecraft on a SPOT VM, write world data, then
#                hard-kill the machine via `gcloud compute instances reset`
#                (the closest repeatable proxy for SPOT termination, which GCP
#                will not guarantee a timeslot for). Each cycle verifies the
#                world is intact and times recovery end to end.
#   all        - both, sequentially.
#
# Evidence must be recorded on #545; nothing here survives the spike.
#
# Requires: the spike.sh harness, docker (for the mc-monitor probe), gcloud.

set -u

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE="$HERE/spike.sh"
MANIFEST="$HERE/manifests/minecraft.yaml"
K3S_CONFIG="$HERE/configs/cos-k3s.yaml"
DOCKER_CONFIG="$HERE/configs/cos-docker.yaml"

INSTANCE="spike-k3s"
ZONE="europe-west3-b"
CYCLES="${CYCLES:-3}"

now() { date +%s; }
die() { echo "ERROR: $*" >&2; exit 1; }
info() { echo; echo "==> $*"; }

kubectl_rs() {
  # No stderr suppression: a failing command must be visible, and the caller
  # guards the exit status explicitly. Retries a few times because gcloud
  # compute ssh can return non-zero spuriously even on success.
  local tries=3
  while [ "$tries" -gt 0 ]; do
    if "$SPIKE" ssh --command "sudo KUBECONFIG=/etc/rancher/k3s/k3s.yaml kubectl $*"; then
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

# Vanilla answers the list ping before the world is created, so accepting
# connections is not proof the world exists. World integrity markers live
# in /data/world, so wait for level.dat before starting a write.
wait_world_ready() {
  local tries="${1:-60}"
  for _ in $(seq 1 "$tries"); do
    kubectl_rs exec "$(pod_name)" -- sh -c 'test -f /data/world/level.dat' 2>/dev/null && return 0
    sleep 10
  done
  return 1
}

# External handshake on 25565 via mc-monitor: the same probe a real client
# would perform, so "ready" means the full path node IP -> ServiceLB -> pod
# -> JVM accepts connections.
wait_accepting() {
  local ip="$1" tries="${2:-80}"
  for _ in $(seq 1 "$tries"); do
    docker run --rm -q itzg/mc-monitor status --host "$ip" --port 25565 >/dev/null 2>&1 && return 0
    sleep 10
  done
  return 1
}

# --- baseline: current systemd + docker runtime ------------------------------

cmd_baseline() {
  info "baseline: current runtime (systemd + docker) cold boot on COS"
  "$SPIKE" create --os cos --user-data "$DOCKER_CONFIG"
  local ip; ip="$(external_ip)"
  local t0
  t0="$(now)"   # instance is RUNNING once create returns
  if wait_accepting "$ip" 90; then
    printf 'BASELINE_ACCEPT_SECONDS=%s\n' "$(( $(now) - t0 ))"
  else
    printf 'BASELINE_ACCEPT_SECONDS=TIMEOUT\n'
  fi
  "$SPIKE" destroy >/dev/null
  info "baseline done"
}

# --- k8s preemption test ------------------------------------------------------

cmd_k8s() {
  info "=== phase 0: create SPOT VM and bootstrap k3s ==="
  "$SPIKE" create --os cos --user-data "$K3S_CONFIG"
  local ip; ip="$(external_ip)"
  info "external IP: $ip"

  wait_instance_running || die "instance never reached RUNNING"

  local t0 t1 boot_time
  t0="$(now)"
  wait_node_ready || die "k3s node never Ready on first boot"
  info "k3s node Ready after $(( $(now) - t0 ))s"

  info "applying manifests"
  cat "$MANIFEST" | "$SPIKE" ssh --command 'sudo KUBECONFIG=/etc/rancher/k3s/k3s.yaml kubectl apply -f -'

  wait_pod_ready || die "minecraft pod never Ready on first boot"
  wait_accepting "$ip" 80 || die "minecraft never accepting on first boot"
  t1="$(now)"
  boot_time=$((t1 - t0))
  printf 'K8S_FIRST_BOOT_SECONDS=%s\n' "$boot_time"
  wait_world_ready || die "world never created on first boot"

  info "=== phase 1: hard-kill cycles ==="
  local cycle count="$CYCLES"
  : > "$HERE/.preemption-results.txt"

  for cycle in $(seq 1 "$count"); do
    info "--- cycle $cycle/$count ---"

    local pod; pod="$(pod_name)"
    [ -n "$pod" ] || die "cannot find minecraft pod for write test"

    local tag; tag="cycle-$cycle-$(date +%s)"
    info "writing integrity marker $tag"
    kubectl_rs exec "$pod" -- sh -c "echo '$tag' > /data/world/.metio-preempt-marker && sync" \
      || die "marker write failed (cycle $cycle)"

    # Cycle 2 demonstrates the hard kill landing DURING an active write: a dd
    # stream is left running against the world file and the machine is killed
    # a moment later, before the write can complete.
    if [ "$cycle" -eq 2 ]; then
      info "starting 256Mi dd against the world, resetting 1s later (write in flight)"
      kubectl_rs exec "$pod" -- sh -c 'dd if=/dev/zero of=/data/world/.metio-preempt-big bs=1M count=256 2>/dev/null &' \
        || die "failed to start dd (cycle $cycle)"
      sleep 1
    fi

    info "RESETTING instance (simulated preemption)"
    local t_w t_t recovery
    t_w="$(now)"
    gcloud compute instances reset "$INSTANCE" --zone="$ZONE" >/dev/null

    wait_instance_running || die "instance never returned to RUNNING after reset"
    wait_node_ready || die "k3s node never Ready after reset (cycle $cycle)"
    wait_pod_ready || die "minecraft pod never Ready after reset (cycle $cycle)"
    wait_accepting "$ip" 80 || die "minecraft never accepting after reset (cycle $cycle)"
    t_t="$(now)"
    recovery=$((t_t - t_w))

    local new_pod marker_ok big_bytes regions
    new_pod="$(pod_name)"
    marker_ok="$(kubectl_rs exec "$new_pod" -- sh -c 'cat /data/world/.metio-preempt-marker 2>/dev/null' || echo MISSING)"
    big_bytes="$(kubectl_rs exec "$new_pod" -- sh -c 'wc -c < /data/world/.metio-preempt-big 2>/dev/null || echo 0')"
    regions="$(kubectl_rs exec "$new_pod" -- sh -c 'ls /data/world/region/*.mca 2>/dev/null | wc -l')"

    local status="PASS"
    [ "$marker_ok" = "$tag" ] || status="FAIL"

    printf 'cycle=%s recovery_seconds=%s marker=%s bigfile_bytes=%s region_files=%s status=%s\n' \
      "$cycle" "$recovery" "$marker_ok" "$big_bytes" "$regions" "$status" >> "$HERE/.preemption-results.txt"

    info "recovery: ${recovery}s end-to-end (reset -> accepting)  status=$status"
    info "world integrity: marker=$marker_ok  big-file=$big_bytes bytes  region-files=$regions"
  done

  info "=== phase 2: summary ==="
  echo
  echo "First boot (no preemption): ${boot_time}s to accepting connections"
  cat "$HERE/.preemption-results.txt"

  "$SPIKE" destroy >/dev/null
  echo
  echo "Teardown complete. Verify with: $SPIKE list"
}

cmd_all() {
  cmd_baseline
  cmd_k8s
}

usage() {
  cat <<'EOF'
SPIKE ONLY - preemption survivability test for #545.

Usage: test-preemption.sh baseline|k8s|all

  baseline  Time cold boot of the current systemd+docker runtime (comparison)
  k8s       Boot k3s+Minecraft on SPOT, hard-kill CYCLES times, verify+time
  all       baseline then k8s

Env: CYCLES=3  (number of hard-kill cycles; override for iteration)
EOF
}

main() {
  local cmd="${1:-}"
  case "$cmd" in
    baseline) cmd_baseline ;;
    k8s)      cmd_k8s ;;
    all)      cmd_all ;;
    *)        usage; exit 1 ;;
  esac
}

main "$@"