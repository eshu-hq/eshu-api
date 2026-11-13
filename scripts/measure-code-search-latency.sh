#!/usr/bin/env bash
# W4 Code search termination latency gate (Refs eshu-hq/eshu#9669,
# satellite eshu-hq/eshu-api#113, D13 tracker eshu-hq/eshu#10491).
#
# Owns the read-latency comparison the census assigns the satellite:
# core keeps the contract-registry and static-contract gates; this repo
# proves the terminated W4 Code search reads are not slower than the
# relay they replace. Hermetic by design: both legs run against the same
# in-process stub core, so the numbers isolate dest-side cost (wave
# gate + family authorizer + telemetry), not core or network time. The
# pinned-core-image leg against a real deployment rides flip prep
# (D11); this gate runs on every PR and in CI.
#
# Method: go test benchmarks BenchmarkCodeSearchTerminatedPOST (one leg
# per wave route) vs BenchmarkCodeSearchRelayPOST (bundles-relay, the
# still-relayed code POST no wave table covers, on the generic proxy),
# -count=5, take the median ns/op. FAIL when terminated exceeds relay by
# more than 50% on any leg (generous: every leg does one server-side
# fetch, so any real pathology — a double fetch, a retry, a body
# re-read — exceeds it loudly while scheduling noise does not).
#
# Usage: bash scripts/measure-code-search-latency.sh
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

term_code_search="$(bench 'BenchmarkCodeSearchTerminatedPOST/code-search' | median)"
term_symbols_search="$(bench 'BenchmarkCodeSearchTerminatedPOST/symbols-search' | median)"
relay_post="$(bench 'BenchmarkCodeSearchRelayPOST/bundles-relay' | median)"

check_leg code-search "$term_code_search" "$relay_post"
check_leg symbols-search "$term_symbols_search" "$relay_post"
