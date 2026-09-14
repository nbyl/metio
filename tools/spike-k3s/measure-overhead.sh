#!/usr/bin/env bash
#
# SPIKE ONLY - ADR-0008 issue #546.
#
# Measures idle resource overhead on a COS machine type with and without k3s,
# so the control-plane cost can be weighed against ADR-0006's MAX_MEMORY
# percentage (issue #533).
#
# Scenarios:
#   bare - plain COS, no user-data. The OS baseline the k3s overhead is the
#          difference against.
#   k3s  - the existing cos-k3s.yaml config (traefik + metrics-server
#          disabled, ServiceLB retained). NO minecraft workload applied, so
#          only control-plane + Kubernetes system pods are running.
#
# A run takes ~15-20 min (a 10 min idle CPU window is sustained between two
# memory snapshots). Use --no-spot: an interrupted window invalidates the
# sample, and this ticket is not about preemption.
#
# Evidence: the log is captured under .overhead-<type>-<scenario>.log and a
# summary row is appended to overhead-results.txt. Nothing here survives the
# spike (#542).

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPIKE="$HERE/spike.sh"
K3S_CONFIG="$HERE/configs/cos-k3s.yaml"
RESULTS="$HERE/overhead-results.txt"

ZONE="europe-west3-b"
TYPE="e2-medium"
SUSTAIN="${SUSTAIN:-600}"     # seconds of idle CPU sampling
INTERVAL="${INTERVAL:-10}"    # sampling interval during the window
SETTLE="${SETTLE:-120}"       # extra quiet time before sampling starts

now() { date +%s; }
die() { echo "ERROR: $*" >&2; exit 1; }
info() { echo; echo "==> $*"; }

# Remote measurement script. Runs through `bash -s` on stdin so no exec bit is
# needed anywhere (all of /var, /tmp are noexec on COS). Everything reads
# /proc; there is no pidstat or metrics-server dependency.
IFS= read -r -d '' REMOTE <<'REMOTE' || true
set -euo pipefail
scenario="$1"; settle="$2"; sustain="$3"; interval="$4"
K="sudo KUBECONFIG=/etc/rancher/k3s/k3s.yaml kubectl"

die() { echo "ERR: $*" >&2; exit 1; }

match_name() {
  # Processes that are part of the always-running stack. k3s embeds containerd
  # and etcd inside k3s-server; local-path/svclb/coredns are k3s's own system
  # pods and run on every node regardless of workload. comm is truncated to
  # 15 chars, so match prefixes.
  case "$1" in
    k3s-server|kubelet|containerd|containerd-shim|agent|k3s-*|coredns|local-path*|svclb-*|storage-provisioner*)
      return 0 ;;
    *) return 1 ;;
  esac
}

proc_list() {
  for p in /proc/[0-9]*; do
    pid="${p#/proc/}"
    name="$(awk '/^Name:/{print $2}' "$p/status" 2>/dev/null || true)"
    [ -n "$name" ] || continue
    if match_name "$name"; then echo "$pid $name"; fi
  done
}

mem_snapshot() {
  label="$1"
  echo "== $label =="
  echo "--- meminfo ---"
  grep -E '^(MemTotal|MemFree|MemAvailable|Buffers|Cached|SwapTotal|SwapFree):' /proc/meminfo
  echo "--- processes (pid name rss_kb) ---"
  proc_list | while read -r pid name; do
    rss="$(awk '/^VmRSS:/{print $2}' "/proc/$pid/status" 2>/dev/null || echo 0)"
    [ -n "$rss" ] || rss=0
    echo "$pid $name $rss"
  done
  echo "--- df ---"
  df -m /mnt/disks/metio 2>/dev/null || echo "no /mnt/disks/metio"
}

# Sample /proc/pid/stat ticks each interval; report per-process min/avg/max
# CPU% of machine-total plus host idle/steal totals.
cpu_window() {
  start="$(date +%s)"
  samples="/tmp/samples-$start"
  : > "$samples"

  # prime previous ticks + host counters (cpu line: user nice system idle
  # iowait irq softirq steal guest guest_nice => fields 2..9)
  while read -r pid name; do
    prev["$pid"]="$(awk '{print $14+$15}' "/proc/$pid/stat" 2>/dev/null || echo 0)"
  done < <(proc_list)
  idle1="$(awk '/^cpu /{print $5}' /proc/stat)"
  total1="$(awk '/^cpu /{print $2+$3+$4+$5+$6+$7+$8+$9}' /proc/stat)"

  n=0
  while :; do
    sleep "$interval"
    nowt="$(date +%s)"
    kept=$(( (nowt-start) / interval ))

    idle2="$(awk '/^cpu /{print $5}' /proc/stat)"
    total2="$(awk '/^cpu /{print $2+$3+$4+$5+$6+$7+$8+$9}' /proc/stat)"
    dt=$((total2-total1))
    idle_dt=$((idle2-idle1))
    idle1="$idle2"; total1="$total2"

    # per-process instant CPU% of machine-total: delta_proc_ticks / delta_all * 100
    while read -r pid name; do
      nowv="$(awk '{print $14+$15}' "/proc/$pid/stat" 2>/dev/null || echo 0)"
      prevv="${prev[$pid]:-0}"
      prev["$pid"]="$nowv"
      if [ "$dt" -gt 0 ]; then
        awk -v d="$((nowv-prevv))" -v t="$dt" 'BEGIN{printf "%.2f", (d/t)*100}' >> "$samples"
        printf ' %s %s\n' "$name" "$pid" >> "$samples"
      else
        printf '0.00 %s %s\n' "$name" "$pid" >> "$samples"
      fi
    done < <(proc_list)

    n=$((n+1))
    [ "$kept" -ge "$((sustain/interval))" ] && break
  done

  steal_total="$(awk '/^cpu /{print $9}' /proc/stat)"
  echo "total_samples=$n sustain=$sustain interval=$interval host_idle_delta_ticks=$idle_dt host_steal_total_ticks=$steal_total"
  awk -v n="$n" '{
    cpu=$1+0; name=$2; pid=$3
    acc[pid]+=cpu; cnt[pid]++
    if (cpu > max[pid]+0 || max[pid]=="") max[pid]=cpu
    if (min[pid]=="" || cpu < min[pid]+0) min[pid]=cpu
    nm[pid]=name
  } END {
    for (p in acc) printf "pid:%s name:%s mean:%.2f min:%.2f max:%.2f\n", p, nm[p], acc[p]/cnt[p], min[p], max[p]
  }' "$samples" | sort -t: -k1,1
  rm -f "$samples"
}

ok=0
if [ "$scenario" = k3s ]; then
  echo "waiting for k3s node Ready"
  for i in $(seq 1 60); do
    if $K get node --no-headers 2>/dev/null | grep -q ' Ready '; then ok=1; break; fi
    sleep 5
  done
  [ "$ok" = 1 ] || die "k3s node never Ready"

  echo "waiting for all system pods Running/Completed"
  ok=0
  for i in $(seq 1 90); do
    bad="$($K get pods -A --no-headers 2>/dev/null | awk '$4!="Running" && $4!="Completed"{n++} END{print n+0}')"
    [ "${bad:-1}" -eq 0 ] && ok=1 && break
    sleep 5
  done
  [ "$ok" = 1 ] || { echo "pods never settled:"; $K get pods -A --no-headers; die "pods not settled"; }
  echo "--- cluster start view ---"
  $K get node --no-headers
  $K get pods -A --no-headers
else
  # bare: nothing to wait for beyond ssh being up
  echo "bare scenario: no cluster to wait for"
fi

echo "settling ${settle}s before sampling"
sleep "$settle"

mem_snapshot "memory-snapshot-A"
echo "sampling CPU for ${sustain}s (interval ${interval}s)"
cpu_window
mem_snapshot "memory-snapshot-B"

echo "@DONE"
REMOTE

# Wait until ssh is actually usable. The instance reports RUNNING before
# OS Login/sshd finishes coming up, and a single early connect fails (255).
wait_ssh() {
  local tries="${1:-40}"
  for _ in $(seq 1 "$tries"); do
    if "$SPIKE" ssh --command 'echo up' >/dev/null 2>&1; then return 0; fi
    sleep 5
  done
  return 1
}

run_remote() {
  local scenario="$1"
  wait_ssh || die "ssh never came up on $TYPE/$scenario"
  # `bash -s` leaves $0 as bash, so positional params start at $1.
  printf '%s\n' "$REMOTE" | "$SPIKE" ssh --command "bash -s '$scenario' '$SETTLE' '$SUSTAIN' '$INTERVAL'" \
    | tee "$HERE/.overhead-${TYPE}-${scenario}.log"
}

run_scenario() {
  local scenario="$1"
  info "=== scenario: $scenario on $TYPE (no-spot, steady window) ==="
  "$SPIKE" destroy >/dev/null 2>&1 || true

  local -a args=(create --machine-type "$TYPE" --zone "$ZONE" --no-spot)
  [ "$scenario" = k3s ] && args+=(--user-data "$K3S_CONFIG")
  "$SPIKE" "${args[@]}"

  local t0
  t0="$(now)"
  run_remote "$scenario"
  grep -q '@DONE' "$HERE/.overhead-${TYPE}-${scenario}.log" || die "measurement script did not complete"

  info "measurement done ($(( $(now) - t0 ))s wall); summarising"
  emit_summary "$scenario"

  "$SPIKE" destroy >/dev/null
  info "teardown done"
}

emit_summary() {
  local scenario="$1" log="$HERE/.overhead-${TYPE}-${scenario}.log"

  local memtotal_kb avail_a avail_b ctrl_a ctrl_b used_a used_b pctrl_kb
  memtotal_kb="$(grep '^MemTotal:'  "$log" | head -1 | awk '{print $2}')"
  avail_a="$(grep '^MemAvailable:' "$log" | head -1 | awk '{print $2}')"
  avail_b="$(grep '^MemAvailable:' "$log" | tail -1 | awk '{print $2}')"
  ctrl_a="$(sed -n '/memory-snapshot-A/,/--- df ---/p' "$log" | awk '$3~/^[0-9]+$/{s+=$3} END{print s+0}')"
  ctrl_b="$(sed -n '/memory-snapshot-B/,/--- df ---/p' "$log" | awk '$3~/^[0-9]+$/{s+=$3} END{print s+0}')"
  pctrl_kb="$(( (ctrl_a+ctrl_b)/2 ))"

  used_a=$(( (memtotal_kb-avail_a)/1024 ))
  used_b=$(( (memtotal_kb-avail_b)/1024 ))
  local used_mb=$(( (used_a+used_b)/2 ))
  local used_pct ctrl_mb ctrl_pct
  used_pct="$(awk -v b="$((used_mb*1024))" -v m="$memtotal_kb" 'BEGIN{printf "%.1f", b/m*100}')"
  ctrl_mb=$(( pctrl_kb/1024 ))
  ctrl_pct="$(awk -v b="$pctrl_kb" -v m="$memtotal_kb" 'BEGIN{printf "%.1f", b/m*100}')"

  local cpu_mean cpu_max
  cpu_mean="$(grep '^pid:' "$log" | awk '{for(i=1;i<=NF;i++) if($i~/^mean:/){split($i,a,":"); s+=a[2]}} END{if(s) printf "%.2f", s; else printf "0"}')"
  cpu_max="$(grep '^pid:' "$log" | awk 'BEGIN{m=0} {for(i=1;i<=NF;i++) if($i~/^max:/){split($i,a,":"); if(a[2]+0>m+0) m=a[2]+0}} END{printf "%.2f", m}')"

  # header once
  [ -f "$RESULTS" ] || {
    printf '%-21s %-10s %9s %8s %8s %7s %12s %13s %12s\n' \
      scenario machine-type used_mb used_pct ctrl_mb ctrl_pct cpu_sum_%total cpu_peak_%total memtotal_kb > "$RESULTS"
  }
  printf '%-21s %-10s %9s %8s %8s %7s %12s %13s %12s\n' \
    "$scenario" "$TYPE" "$used_mb" "$used_pct" "$ctrl_mb" "$ctrl_pct" "$cpu_mean" "$cpu_max" "$memtotal_kb" >> "$RESULTS"
  cat "$RESULTS"
}

usage() {
  sed -n '2,8p' "$0"
  cat <<EOF

Usage: measure-overhead.sh {bare|k3s|all} [--machine-type TYPE]

  bare  Plain COS, no user-data (OS baseline)
  k3s   cos-k3s.yaml, no minecraft workload (control-plane overhead)
  all   bare then k3s on the given machine type

Flags:
  --machine-type TYPE   Machine type (default: e2-medium)

Env: SUSTAIN=600 (CPU window s)  INTERVAL=10  SETTLE=120

Output: overhead-results.txt + .overhead-<type>-<scenario>.log
EOF
}

main() {
  local cmd="${1:-}"; shift || true
  while [ $# -gt 0 ]; do
    case "$1" in
      --machine-type) TYPE="${2:?--machine-type needs a value}"; shift 2 ;;
      *) die "unknown flag '$1' (try --help)" ;;
    esac
  done
  case "$cmd" in
    bare) run_scenario bare ;;
    k3s)  run_scenario k3s ;;
    all)  run_scenario bare; run_scenario k3s ;;
    -h|--help|help) usage; exit 0 ;;
    *) die "unknown command '$cmd' (expected bare|k3s|all)" ;;
  esac
}

main "$@"