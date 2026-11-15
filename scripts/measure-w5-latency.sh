#!/usr/bin/env bash
# Satellite W5 entity/impact/story/packet termination latency gate (Refs
# eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#97).
#
# Owns the read-latency comparison the census assigns the satellite:
# core keeps the contract-registry and static-contract gates; this repo
# proves the terminated W5 reads are not slower than the relay they
# replace. Hermetic by design: all legs run against the same
# in-process stub core, so the numbers isolate dest-side cost (wave
# gate + family authorizer + telemetry), not core or network time. The
# pinned-core-image leg against a real deployment rides flip prep
# (D11); this gate runs on every PR and in CI.
#
# Method: go test benchmarks BenchmarkEntityTerminatedRead (resolve
# POST + one story GET), BenchmarkImpactTerminatedRead (trace POST),
# BenchmarkStoryTerminatedRead (story GET),
# BenchmarkPacketTerminatedRead (packet GET) vs relay legs
# (BenchmarkEntityRelayRead hold-get-relay/get-relay/post-relay,
# BenchmarkImpactRelayRead holds + post-relay — still-relayed or
# hold routes on the explicit relay stack or the generic proxy),
# -count=5, take the median ns/op. FAIL when a terminated leg exceeds
# its same-method relay leg by more than 50% (generous: every leg does
# one server-side fetch, so any real pathology — a double fetch, a
# retry, a body re-read — exceeds it loudly while scheduling noise
# does not).
#
# Usage: bash scripts/measure-w5-latency.sh
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

entity_post="$(bench 'BenchmarkEntityTerminatedRead/entities-resolve' | median)"
entity_get="$(bench 'BenchmarkEntityTerminatedRead/workloads-wl_1-story' | median)"
impact_post="$(bench 'BenchmarkImpactTerminatedRead/trace-deployment-chain' | median)"
story_get="$(bench 'BenchmarkStoryTerminatedRead/story-get' | median)"
packet_get="$(bench 'BenchmarkPacketTerminatedRead/packet-get' | median)"
relay_hold_get="$(bench 'BenchmarkEntityRelayRead/hold-get-relay' | median)"
relay_get="$(bench 'BenchmarkEntityRelayRead/get-relay' | median)"
relay_post="$(bench 'BenchmarkEntityRelayRead/post-relay' | median)"
impact_hold_surface="$(bench 'BenchmarkImpactRelayRead/hold-change-surface' | median)"
impact_hold_investigate="$(bench 'BenchmarkImpactRelayRead/hold-change-surface-investigate' | median)"

check_leg entity-post "$entity_post" "$relay_post"
check_leg entity-get "$entity_get" "$relay_hold_get"
check_leg impact-post "$impact_post" "$relay_post"
check_leg story-get "$story_get" "$relay_get"
check_leg packet-get "$packet_get" "$relay_get"
check_leg impact-hold-surface "$impact_hold_surface" "$relay_post"
check_leg impact-hold-investigate "$impact_hold_investigate" "$relay_post"
