#!/usr/bin/env bash
# Satellite divergence termination latency gate (Refs eshu-hq/eshu#9669,
# satellite eshu-hq/eshu-api#125, D13 tracker eshu-hq/eshu#10491).
#
# Owns the read-latency comparison the census assigns the satellite:
# core keeps the contract-registry and static-contract gates; this repo
# proves the terminated divergence reads are not slower than the relay
# they replace. Hermetic by design: all legs run against the same
# in-process stub core, so the numbers isolate dest-side cost (wave
# gate + family authorizer + telemetry), not core or network time. The
# pinned-core-image leg against a real deployment rides flip prep
# (D11); this gate runs on every PR and in CI.
#
# Method: go test benchmarks BenchmarkCodeDivergenceTerminatedRead (one
# leg per wave route: findings-post, investigate-post, report-post) vs
# BenchmarkCodeDivergenceRelayRead (post-relay — a still-relayed POST
# no wave table covers, on the generic proxy), -count=5, take the median
# ns/op. FAIL when a terminated leg exceeds the relay leg by more than
# 50% (generous: every leg does one server-side fetch, so any real
# pathology — a double fetch, a retry, a body re-read — exceeds it
# loudly while scheduling noise does not).
#
# Usage: bash scripts/measure-code-divergence-latency.sh
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

term_findings_post="$(bench 'BenchmarkCodeDivergenceTerminatedRead/findings-post' | median)"
term_investigate_post="$(bench 'BenchmarkCodeDivergenceTerminatedRead/investigate-post' | median)"
term_report_post="$(bench 'BenchmarkCodeDivergenceTerminatedRead/report-post' | median)"
relay_post="$(bench 'BenchmarkCodeDivergenceRelayRead/post-relay' | median)"

check_leg findings-post "$term_findings_post" "$relay_post"
check_leg investigate-post "$term_investigate_post" "$relay_post"
check_leg report-post "$term_report_post" "$relay_post"
