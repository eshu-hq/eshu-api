#!/usr/bin/env bash
# W7-writers admin sign-in-policy PATCH latency gate (Refs eshu-hq/eshu#9669,
# D13 tracker eshu-hq/eshu#10491, satellite eshu-hq/eshu-api#114).
#
# Owns the write-latency comparison the census assigns the satellite:
# core keeps the contract-registry and static-contract gates; this repo
# proves the terminated admin sign-in-policy PATCH writer is not slower
# than the relay it replaces. Hermetic by design: both legs run against
# the same in-process stub core, so the numbers isolate dest-side cost
# (family writer gate + family authorizer + telemetry), not core or
# network time. The pinned-core-image leg against a real deployment
# rides flip prep (D11); this evidence runs via
# .github/workflows/read-latency.yml on lane-affecting PRs and weekly,
# and locally with this script.
#
# Method: go test benchmarks BenchmarkAdminSignInPolicyWriteThrough
# (exact path and trailing-slash twin) vs
# BenchmarkAdminSignInPolicyRelayWrite (api package), -count=5, take
# the median ns/op. FAIL when terminated exceeds relay by more than
# 50% on any leg (generous: every leg does one server-side fetch, so
# any real pathology — a double fetch, a retry, a body re-read —
# exceeds it loudly while scheduling noise does not).
#
# Relay-route disclosure: the terminated legs compare against the
# still-relaying PATCH /api/v0/code/bundles; the admin PATCH now
# terminates on the family lane, so every lane gates
# terminated-vs-relay again instead of terminated-vs-terminated.
#
# Usage: bash scripts/measure-signinpolicy-admin-write-latency.sh
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

term_exact="$(bench 'BenchmarkAdminSignInPolicyWriteThrough/exact' | median)"
term_twin="$(bench 'BenchmarkAdminSignInPolicyWriteThrough/twin' | median)"
relay_patch="$(bench 'BenchmarkAdminSignInPolicyRelayWrite/patch' | median)"

check_leg exact "$term_exact" "$relay_patch"
check_leg twin "$term_twin" "$relay_patch"

echo "signinpolicy-admin-write-latency: all legs within budget"
