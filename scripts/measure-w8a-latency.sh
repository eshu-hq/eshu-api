#!/usr/bin/env bash
# W8a satellite termination latency gate (Refs eshu-hq/eshu#9669,
# satellite eshu-hq/eshu-api#100).
#
# Owns the write-latency comparison the census assigns the satellite:
# core keeps the contract-registry and static-contract gates; this repo
# proves the terminated W8a reads are not slower than the relay they
# replace. Hermetic by design: both legs run against the same
# in-process stub core, so the numbers isolate dest-side cost (wave
# gate + family authorizer + telemetry), not core or network time. The
# pinned-core-image leg against a real deployment rides flip prep.
#
# Method: go test benchmarks BenchmarkW8aTerminatedRead (one leg per
# wave family) vs BenchmarkW8aRelayRead (api package), -count=5, take
# the median ns/op. FAIL when terminated exceeds relay by more than
# 50% on any leg. Generous: every leg does one server-side fetch, so
# any real pathology (a double fetch, a retry, a body re-read) exceeds
# it loudly while scheduling noise does not.
#
# Relay-route disclosure (the W8a lane terminates no earlier relay
# leg): terminated legs are the 8 per-family W8a representatives vs
# relay GET /api/v0/dependencies (the D2-D4-pinned relay read leg)
# and relay POST /api/v0/code/bundles (the D3/D4-pinned relay POST
# leg). GET legs gate against the relay GET, POST legs against the
# relay POST.
#
# Usage: bash scripts/measure-w8a-latency.sh
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

term_secrets="$(bench 'BenchmarkW8aTerminatedRead/secrets-iam' | median)"
term_status="$(bench 'BenchmarkW8aTerminatedRead/status' | median)"
term_iac_get="$(bench 'BenchmarkW8aTerminatedRead/iac-get' | median)"
term_metrics="$(bench 'BenchmarkW8aTerminatedRead/metrics' | median)"
term_iac_post="$(bench 'BenchmarkW8aTerminatedRead/iac-post' | median)"
term_semantic="$(bench 'BenchmarkW8aTerminatedRead/semantic' | median)"
term_terraform="$(bench 'BenchmarkW8aTerminatedRead/terraform' | median)"
term_aws="$(bench 'BenchmarkW8aTerminatedRead/aws-drift' | median)"
relay_get="$(bench 'BenchmarkW8aRelayRead/get' | median)"
relay_post="$(bench 'BenchmarkW8aRelayRead/post' | median)"

check_leg secrets-iam "$term_secrets" "$relay_get"
check_leg status "$term_status" "$relay_get"
check_leg iac-get "$term_iac_get" "$relay_get"
check_leg metrics "$term_metrics" "$relay_get"
check_leg iac-post "$term_iac_post" "$relay_post"
check_leg semantic "$term_semantic" "$relay_post"
check_leg terraform "$term_terraform" "$relay_post"
check_leg aws-drift "$term_aws" "$relay_post"

echo "w8a-latency: all legs within budget"
