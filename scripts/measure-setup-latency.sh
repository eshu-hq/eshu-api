#!/usr/bin/env bash
# D10 setup-latency gate (Refs eshu-hq/eshu#9669, part of #6707).
#
# Owns the latency comparison the census assigns the satellite: the
# terminated setup-state GET (via the SetupNeeded port) and the
# terminated-writer setup claim POST (write-through transport) must not
# be slower than the
# relay they replace. Hermetic by design: every leg runs against the
# same in-process stub core, so the numbers isolate dest-side cost
# (explicit route match plus, on the state leg, the single port fetch
# and the synthesized 200 — and on the claim leg, the write-through
# forward), not core or network time. The pinned-core-image leg
# against a real deployment rides flip prep (D11); this gate runs on
# every PR and in CI.
#
# Method: go test benchmarks BenchmarkSetupState /
# BenchmarkSetupWrite vs BenchmarkSetupRelayWrite /
# BenchmarkSetupRelayRead (api package), -count=5, take the median
# ns/op. FAIL when a setup leg exceeds relay by more than 50%.
# Generous: every leg does one server-side fetch, so any real
# pathology (a double fetch, a retry, a body re-read) exceeds it
# loudly while scheduling noise does not.
#
# Relay-route disclosure: the write-relay leg is POST
# /api/v0/code/bundles, which stays on the generic relay (the
# D3/D4/Leaf B lanes pin it as their relay POST leg; the Compare POST
# terminated in its gap-04 lane); the
# read-relay leg is GET /api/v0/dependencies, which stays on the
# generic relay. The setup paths left the relay on the D10 wave (the
# state GET terminates; the claim POST rides the terminated-writer
# transport); the
# legs compare different routes by necessity, and every leg does
# exactly one server-side fetch, so the 1.50 bound stays a real
# pathology check either way.
#
# Usage: bash scripts/measure-setup-latency.sh
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
  local leg="$1" clone="$2" relay="$3"
  if [[ -z "$clone" || -z "$relay" ]]; then
    echo "FAIL: no benchmark samples for leg ${leg}" >&2
    exit 1
  fi
  # shellcheck disable=SC2004
  local ratio
  ratio="$(awk -v t="$clone" -v r="$relay" 'BEGIN {printf "%.2f", t/r}')"
  local over
  over="$(awk -v t="$clone" -v r="$relay" 'BEGIN {print (t > r*1.5) ? 1 : 0}')"
  echo "leg ${leg}: clone=${clone}ns relay=${relay}ns ratio=${ratio} (fail past 1.50)"
  if [[ "$over" == "1" ]]; then
    echo "FAIL: clone leg ${leg} regresses past 1.50x of relay" >&2
    exit 1
  fi
}

clone_state="$(bench 'BenchmarkSetupState/state' | median)"
clone_claim="$(bench 'BenchmarkSetupWrite/claim' | median)"
relay_post="$(bench 'BenchmarkSetupRelayWrite/post' | median)"
relay_get="$(bench 'BenchmarkSetupRelayRead/get' | median)"

check_leg state "$clone_state" "$relay_get"
check_leg claim "$clone_claim" "$relay_post"

echo "setup-latency: all legs within budget"
