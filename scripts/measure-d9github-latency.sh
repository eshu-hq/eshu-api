#!/usr/bin/env bash
# D9 GitHub-login clone-latency gate (Refs eshu-hq/eshu#9669, part of #6707).
#
# Owns the read-latency comparison the census assigns the satellite:
# core keeps the contract-registry and static-contract gates; this repo
# proves the tier-C GitHub-login clones are not slower than the relay
# they replace. Hermetic by design: both legs run against the same
# in-process stub core, so the numbers isolate dest-side cost
# (explicit route match + telemetry), not core or network time. The
# pinned-core-image leg against a real deployment rides flip prep
# (D11); this gate runs on every PR and in CI.
#
# Method: go test benchmarks BenchmarkD9GitHubPublicLogin /
# BenchmarkD9GitHubPublicCallback (api package) vs BenchmarkD8RelayWrite /
# BenchmarkD8RelayRead, -count=5, take the median ns/op. FAIL when a
# clone leg exceeds relay by more than 50%. Generous: both legs do one
# server-side fetch, so any real pathology (a double fetch, a retry, a
# body re-read) exceeds it loudly while scheduling noise does not.
#
# Relay-route disclosure: the write-relay leg is POST
# /api/v0/code/bundles, which stays on the generic relay (the
# D3/D4/Leaf B lanes pin it as their relay POST leg; the Compare POST
# terminated in its gap-04 lane); the read-relay leg is GET
# /api/v0/dependencies, which stays on the generic relay. The D9
# paths now clone; both legs do exactly one server-side fetch, so the
# 1.50 bound stays a real pathology check either way.
#
# Usage: bash scripts/measure-d9github-latency.sh
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

clone_login_bare="$(bench 'BenchmarkD9GitHubPublicLogin/bare' | median)"
clone_login_query="$(bench 'BenchmarkD9GitHubPublicLogin/query' | median)"
clone_callback="$(bench 'BenchmarkD9GitHubPublicCallback/callback' | median)"
relay_post="$(bench 'BenchmarkD8RelayWrite/post' | median)"
relay_get="$(bench 'BenchmarkD8RelayRead/get' | median)"

check_leg login-bare "$clone_login_bare" "$relay_get"
check_leg login-query "$clone_login_query" "$relay_get"
check_leg callback "$clone_callback" "$relay_get"
check_leg login-query-vs-post "$clone_login_query" "$relay_post"
check_leg callback-vs-post "$clone_callback" "$relay_post"

echo "d9github-latency: all legs within budget"
