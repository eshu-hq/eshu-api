#!/usr/bin/env bash
# D3 Gap 02 trio read-latency gate (Refs eshu-hq/eshu#9669, satellite
# eshu-hq/eshu-api#70).
#
# Owns the per-route latency comparison the census assigns the
# satellite for the three Gap 02-pinned Admin GETs: core keeps the
# contract-registry and static-contract gates; this repo proves the
# terminated trio reads are not slower than the relay they replace.
# Hermetic by design: all legs run against the same in-process stub
# core, so the numbers isolate dest-side cost (wave gate + family
# authorizer + telemetry), not core or network time. The
# pinned-core-image leg against a real deployment rides flip prep
# (D11); this gate runs on every PR and in CI.
#
# Method: go test benchmarks BenchmarkD3TrioTerminatedRead (one leg per
# trio route: delta-union-completeness, reindex-progress, tuning-report)
# vs BenchmarkD3RelayRead/get (foreign-bearer explicit relay on the
# wave-mounted dependencies route), -count=5, take the median ns/op.
# FAIL when a terminated leg exceeds relay by more than 50%.
# Generous: every leg does one server-side fetch, so any real pathology
# (a double fetch, a retry, a body re-read) exceeds it loudly while
# scheduling noise does not.
#
# Usage: bash scripts/measure-d3trio-latency.sh
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

term_delta="$(bench 'BenchmarkD3TrioTerminatedRead/delta-union-completeness' | median)"
term_reindex="$(bench 'BenchmarkD3TrioTerminatedRead/reindex-progress' | median)"
term_tuning="$(bench 'BenchmarkD3TrioTerminatedRead/tuning-report' | median)"
relay_get="$(bench 'BenchmarkD3RelayRead/get' | median)"

check_leg delta-union-completeness "$term_delta" "$relay_get"
check_leg reindex-progress "$term_reindex" "$relay_get"
check_leg tuning-report "$term_tuning" "$relay_get"

echo "d3trio-latency: all legs within budget"
