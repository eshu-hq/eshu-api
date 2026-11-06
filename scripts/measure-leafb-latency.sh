#!/usr/bin/env bash
# Leaf B latency gate (Refs eshu-hq/eshu#9669, part of #6707).
#
# Owns the read-latency comparison the census assigns the satellite:
# core keeps the contract-registry and static-contract gates; this repo
# proves the terminated wave is not slower than the relay it replaces.
# Hermetic by design: both legs run against the same in-process stub
# core, so the numbers isolate dest-side cost (wave gate + family
# authorizer + telemetry), not core or network time. The
# pinned-core-image leg against a real deployment rides flip prep
# (D11); this gate runs on every PR and in CI.
#
# Method: go test benchmarks BenchmarkLeafBTerminatedRead vs
# BenchmarkLeafBRelayRead (api package), -count=5, take the median
# ns/op per sub-benchmark. FAIL when terminated exceeds relay by more
# than 50% on any leg (generous: both legs do one server-side fetch,
# so any real pathology — a double fetch, a retry, a body re-read —
# exceeds it loudly while scheduling noise does not).
#
# Relay-route disclosure (D4 moved the legs): the relay GET is
# /api/v0/dependencies and the relay POST is /api/v0/code/bundles —
# both still cross the generic proxy (the pre-D4 repositories legs
# terminate natively since the D4 wave).
#
# Usage: bash scripts/measure-leafb-latency.sh
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

term_get="$(bench 'BenchmarkLeafBTerminatedRead/get' | median)"
relay_get="$(bench 'BenchmarkLeafBRelayRead/get' | median)"
term_post="$(bench 'BenchmarkLeafBTerminatedRead/post' | median)"
relay_post="$(bench 'BenchmarkLeafBRelayRead/post' | median)"

check_leg get "$term_get" "$relay_get"
check_leg post "$term_post" "$relay_post"

echo "leafb-latency: all legs within budget"
