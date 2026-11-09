#!/usr/bin/env bash
# Compare relay-leg latency gate (Refs eshu-hq/eshu#9669, satellite
# eshu-hq/eshu-api#72, gap 04).
#
# Owns the write-latency comparison the census assigns the satellite:
# core keeps the contract-registry and static-contract gates; this repo
# proves the terminated Compare POST is not slower than the relay it
# replaces. Hermetic by design: both legs run against the same
# in-process stub core, so the numbers isolate dest-side cost (wave
# gate + family authorizer + telemetry), not core or network time. The
# pinned-core-image leg against a real deployment rides flip prep
# (D11); this gate runs on every PR and in CI.
#
# Method: go test benchmarks BenchmarkCompareTerminatedPOST vs
# BenchmarkCompareRelayPOST (api package), -count=5, take the median
# ns/op. FAIL when terminated exceeds relay by more than 50%.
# Generous: both legs do one server-side fetch, so any real pathology
# (a double fetch, a retry, a body re-read) exceeds it loudly while
# scheduling noise does not.
#
# Relay-route disclosure: the terminated leg is the Compare POST
# (/api/v0/compare/environments), which terminates natively in its
# gap-04 wave gate; the relay leg is POST /api/v0/code/bundles, which
# stays on the generic relay (the D3/D4/Leaf B lanes pin it as their
# relay POST leg).
#
# Usage: bash scripts/measure-compare-latency.sh
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
  local leg="$1" terminated="$2" relay="$3"
  if [[ -z "$terminated" || -z "$relay" ]]; then
    echo "FAIL: no benchmark samples for leg ${leg}" >&2
    exit 1
  fi
  # shellcheck disable=SC2004
  local ratio
  ratio="$(awk -v t="$terminated" -v r="$relay" 'BEGIN {printf "%.2f", t/r}')"
  local over
  over="$(awk -v t="$terminated" -v r="$relay" 'BEGIN {print (t > r*1.5) ? 1 : 0}')"
  echo "leg ${leg}: terminated=${terminated}ns relay=${relay}ns ratio=${ratio} (fail past 1.50)"
  if [[ "$over" == "1" ]]; then
    echo "FAIL: terminated leg ${leg} regresses past 1.50x of relay" >&2
    exit 1
  fi
}

term_post="$(bench 'BenchmarkCompareTerminatedPOST/post' | median)"
relay_post="$(bench 'BenchmarkCompareRelayPOST/post' | median)"

check_leg post "$term_post" "$relay_post"

echo "compare-latency: all legs within budget"
