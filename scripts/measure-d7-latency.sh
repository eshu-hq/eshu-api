#!/usr/bin/env bash
# D7 clone-latency gate (Refs eshu-hq/eshu#9669, part of #6707).
#
# Owns the write-latency comparison the census assigns the satellite:
# core keeps the contract-registry and static-contract gates; this repo
# proves the tier-C admin-four clones are not slower than the relay
# they replace. Hermetic by design: both legs run against the same
# in-process stub core, so the numbers isolate dest-side cost (explicit
# route match + family authorizer + telemetry), not core or network
# time. The pinned-core-image leg against a real deployment rides flip
# prep (D11); this gate runs on every PR and in CI.
#
# Method: go test benchmarks BenchmarkD7NativeWrite vs
# BenchmarkD7RelayWrite (api package), -count=5, take the median
# ns/op. FAIL when a clone leg exceeds relay by more than 50%.
# Generous: both legs do one server-side fetch, so any real pathology
# (a double fetch, a retry, a body re-read) exceeds it loudly while
# scheduling noise does not.
#
# Relay-route disclosure: the relay leg is the Compare POST
# (/api/v0/compare/environments), which stays on the generic relay by
# D6 disclosure (no pin covers its shared graph/content ports). The D6
# lane used GET /api/v0/dependencies for the same reason; both legs do
# exactly one server-side fetch, so the 1.50 bound stays a real
# pathology check either way.
#
# Usage: bash scripts/measure-d7-latency.sh
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

clone_deadletter="$(bench 'BenchmarkD7NativeWrite/dead-letter' | median)"
clone_reindex="$(bench 'BenchmarkD7NativeWrite/reindex' | median)"
relay_post="$(bench 'BenchmarkD7RelayWrite/post' | median)"

check_leg dead-letter "$clone_deadletter" "$relay_post"
check_leg reindex "$clone_reindex" "$relay_post"

echo "d7-latency: all legs within budget"
