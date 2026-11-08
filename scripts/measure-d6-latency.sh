#!/usr/bin/env bash
# D6 read-latency gate (Refs eshu-hq/eshu#9669, part of #6707).
#
# Owns the read-latency comparison the census assigns the satellite:
# core keeps the contract-registry and static-contract gates; this repo
# proves the terminated D6 Status + SupplyChain reads are not slower
# than the relay they replace. Hermetic by design: both legs run against the same
# in-process stub core, so the numbers isolate dest-side cost (wave
# gate + family authorizer + telemetry), not core or network time. The
# pinned-core-image leg against a real deployment rides flip prep
# (D11); this gate runs on every PR and in CI.
#
# Method: go test benchmarks BenchmarkD6TerminatedRead and
# BenchmarkD6ExtraTerminatedRead (phase-2 extra-port legs) vs
# BenchmarkD6RelayRead (api package), -count=5, take the median
# ns/op. FAIL when terminated exceeds relay by more than 50% on any
# leg (generous: every leg does one server-side fetch, so any real
# pathology — a double fetch, a retry, a body re-read — exceeds it
# loudly while scheduling noise does not).
#
# Relay-route disclosure (the D6 wave terminates no earlier relay
# leg): terminated GET /api/v0/status/pipeline, terminated GET
# /api/v0/supply-chain/impact/findings, and the phase-2 terminated
# GET /api/v0/status/operations, GET /api/v0/index-status, and GET
# /api/v0/status/index vs relay GET /api/v0/dependencies. The D2-D4
# relay legs moved to the same still-relaying route in this lane, so
# every lane gates terminated-vs-relay again instead of
# terminated-vs-terminated.
#
# Usage: bash scripts/measure-d6-latency.sh
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

term_status="$(bench 'BenchmarkD6TerminatedRead/status' | median)"
term_supply="$(bench 'BenchmarkD6TerminatedRead/supply' | median)"
term_operations="$(bench 'BenchmarkD6ExtraTerminatedRead/operations' | median)"
term_index_status="$(bench 'BenchmarkD6ExtraTerminatedRead/index-status' | median)"
term_status_index="$(bench 'BenchmarkD6ExtraTerminatedRead/status-index' | median)"
relay_get="$(bench 'BenchmarkD6RelayRead/get' | median)"

check_leg status "$term_status" "$relay_get"
check_leg supply "$term_supply" "$relay_get"
check_leg extra-operations "$term_operations" "$relay_get"
check_leg extra-index-status "$term_index_status" "$relay_get"
check_leg extra-status-index "$term_status_index" "$relay_get"

echo "d6-latency: all legs within budget"
