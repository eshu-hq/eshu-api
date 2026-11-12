#!/usr/bin/env bash
# W3 IaC-6 clone-latency gate (Refs eshu-hq/eshu-api#102, part of
# eshu-hq/eshu#9669).
#
# Owns the read-latency comparison the lane assigns the satellite:
# the four cleared IaC POST legs plus the resources candidate-page GET
# must not be slower than the generic relay they replace. Hermetic by
# design: both legs run against the same in-process stub core, so the
# numbers isolate dest-side cost (explicit route match + telemetry),
# not core or network time. The pinned-core-image leg against a real
# deployment rides flip prep (D11); this gate runs on every PR and in
# CI.
#
# Method: go test benchmarks BenchmarkIaCNativeRead vs
# BenchmarkIaCRelayRead (api package), -count=5, take the median
# ns/op. FAIL when a native leg exceeds relay by more than 50%.
# Generous: both legs do one server-side fetch, so any real pathology
# (a double fetch, a retry, a body re-read) exceeds it loudly while
# scheduling noise does not.
#
# Relay-route disclosure: POST /api/v0/iac/dead stays on the generic
# relay (partial leg: supplemental SearchActive mint + dual-pin dead
# disposition pending), so it is the before-shape here.
#
# Usage: bash scripts/measure-iac-latency.sh
# Needs: go (version in go.mod).
set -euo pipefail

cd "$(dirname "$0")/.."

bench() {
  go test ./api/ -run '^$' -bench "$1" -benchtime=2000x -count=5 2>/dev/null | \
    awk '$1 ~ /^Benchmark/ && $4 == "ns/op" {print $3}'
}

median() {
  sort -n | awk '{a[NR]=$1} END {if (NR%2==1) print a[(NR+1)/2]; else print (a[NR/2]+a[NR/2+1])/2}'
}

check_leg() {
  local leg="$1" clone="$2" relay="$3"
  if [[ -z "$clone" || -z "$relay" ]]; then
    echo "FAIL: no benchmark samples for leg ${leg}" >&2
    exit 1
  fi
  # shellcheck disable=SC2004
  local ratio
  ratio="$(awk -v t="$clone" -v r="$relay" 'BEGIN {printf "%.2f", t/r}')"
  local over
  over="$(awk -v t="$clone" -v r="$relay" 'BEGIN {print (t > r*1.5) ? 1 : 0}')"
  echo "leg ${leg}: clone=${clone}ns relay=${relay}ns ratio=${ratio} (fail past 1.50)"
  if [[ "$over" == "1" ]]; then
    echo "FAIL: clone leg ${leg} regresses past 1.50x of relay" >&2
    exit 1
  fi
}

clone_unmanaged="$(bench 'BenchmarkIaCNativeRead/unmanaged-resources' | median)"
clone_status="$(bench 'BenchmarkIaCNativeRead/management-status' | median)"
clone_explain="$(bench 'BenchmarkIaCNativeRead/management-status-explain' | median)"
clone_candidates="$(bench 'BenchmarkIaCNativeRead/terraform-import-plan-candidates' | median)"
clone_resources="$(bench 'BenchmarkIaCResourcesRead/resources' | median)"
relay_dead="$(bench 'BenchmarkIaCRelayRead/dead-relay' | median)"

check_leg unmanaged-resources "$clone_unmanaged" "$relay_dead"
check_leg management-status "$clone_status" "$relay_dead"
check_leg management-status-explain "$clone_explain" "$relay_dead"
check_leg terraform-import-plan-candidates "$clone_candidates" "$relay_dead"
check_leg resources "$clone_resources" "$relay_dead"

echo "iac-latency: all legs within budget"
