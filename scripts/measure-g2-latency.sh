#!/usr/bin/env bash
# G2 reads termination latency (Refs eshu-hq/eshu#9669, satellite
# eshu-hq/eshu-api#101 RE:G2V2).
#
# Owns the read-latency comparison the census assigns the satellite:
# core keeps the contract-registry and static-contract gates; this repo
# proves the terminated G2 reads are not slower than the relay they
# replace. Hermetic by design: both legs run against the same
# in-process stub core, so the numbers isolate dest-side cost (wave
# gate + family authorizer + telemetry), not core or network time. The
# pinned-core-image leg against a real deployment rides flip prep
# (D11); this evidence runs via .github/workflows/read-latency.yml on
# lane-affecting PRs and weekly, and locally with this script.
#
# Method: go test benchmarks BenchmarkG2TerminatedRead (one leg per
# wave route) vs BenchmarkG2RelayRead (api package), -count=5, take
# the median ns/op. FAIL when terminated exceeds relay by more than
# 50% on any leg (generous: every leg does one server-side fetch,
# so any real pathology — a double fetch, a retry, a body re-read —
# exceeds it loudly while scheduling noise does not).
#
# Relay-route disclosure (the G2 lane terminates no earlier relay
# leg): terminated GETs/POST for the 7 G2 routes vs relay GET
# /api/v0/dependencies. The dependency read stays relay-riding, so
# the lane gates terminated-vs-relay, not terminated-vs-terminated.
#
# Usage: bash scripts/measure-g2-latency.sh
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

term_readiness="$(bench 'BenchmarkG2TerminatedRead/readiness' | median)"
term_readiness_family="$(bench 'BenchmarkG2TerminatedRead/readiness-family' | median)"
term_factschema="$(bench 'BenchmarkG2TerminatedRead/fact-schema' | median)"
term_factschema_kind="$(bench 'BenchmarkG2TerminatedRead/fact-schema-kind' | median)"
term_surface="$(bench 'BenchmarkG2TerminatedRead/surface-inventory' | median)"
term_visualizations="$(bench 'BenchmarkG2TerminatedRead/visualizations-derive' | median)"
term_manifest="$(bench 'BenchmarkG2TerminatedRead/manifest-dependencies' | median)"
relay_get="$(bench 'BenchmarkG2RelayRead/get' | median)"

check_leg readiness "$term_readiness" "$relay_get"
check_leg readiness-family "$term_readiness_family" "$relay_get"
check_leg fact-schema "$term_factschema" "$relay_get"
check_leg fact-schema-kind "$term_factschema_kind" "$relay_get"
check_leg surface-inventory "$term_surface" "$relay_get"
check_leg visualizations-derive "$term_visualizations" "$relay_get"
check_leg manifest-dependencies "$term_manifest" "$relay_get"

echo "g2-latency: all legs within budget"
