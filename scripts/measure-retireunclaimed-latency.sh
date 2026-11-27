#!/usr/bin/env bash
# Workflow-retire-unclaimed clone-latency gate (Refs eshu-hq/eshu#9669,
# part of #6707).
#
# Owns the write-latency comparison the census assigns the satellite:
# core keeps the contract-registry and static-contract gates; this repo
# proves the retire-unclaimed clone is not slower than the relay it
# replaces. Hermetic by design: both legs run against the same
# in-process stub core, so the numbers isolate dest-side cost (explicit
# route match + family authorizer + telemetry), not core or network
# time. The pinned-core-image leg against a real deployment rides flip
# prep (D11); this gate runs on every PR and in CI.
#
# Method: go test benchmarks BenchmarkRetireUnclaimedNativeWrite
# (retire) vs BenchmarkRetireUnclaimedRelayWrite (post —
# foreign-bearer explicit relay on the wave-mounted change-surface
# route), -count=5, take the median ns/op. FAIL when the clone leg
# exceeds relay by more than 50%. Generous: both legs do one
# server-side fetch, so any real pathology (a double fetch, a retry, a
# body re-read) exceeds it loudly while scheduling noise does not.
#
# Usage: bash scripts/measure-retireunclaimed-latency.sh
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

clone_retire="$(bench 'BenchmarkRetireUnclaimedNativeWrite/retire' | median)"
relay_post="$(bench 'BenchmarkRetireUnclaimedRelayWrite/post' | median)"

check_leg retire "$clone_retire" "$relay_post"

echo "retireunclaimed-latency: all legs within budget"
