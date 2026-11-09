#!/usr/bin/env bash
# SAML-triple termination-latency gate (Refs eshu-hq/eshu#9669, satellite
# eshu-hq/eshu-api#72).
#
# Owns the latency comparison the termination lane proves: the
# terminated SAML legs must not be slower than the relay they replace.
# Hermetic by design: every leg runs against the same in-process stub
# core, so the numbers isolate dest-side cost (explicit route match +
# port fetch + response mapping, plus the bounded ACS body read) from
# core or network time.
#
# Method: go test benchmarks BenchmarkSAMLTermMetadata /
# BenchmarkSAMLTermLogin / BenchmarkSAMLTermACS (api package) vs
# BenchmarkD8RelayWrite / BenchmarkD8RelayRead, -count=5, take the
# median ns/op. FAIL when a terminated leg exceeds relay by more than
# 50%. Generous: every leg does one server-side fetch, so any real
# pathology (a double fetch, a retry, a body re-read, a lock convoy
# under duplicate ACS posts) exceeds it loudly while scheduling noise
# does not.
#
# Relay-route disclosure: the write-relay leg is POST
# /api/v0/code/bundles, which stays on the generic relay (the
# D3/D4/Leaf B lanes pin it as their relay POST leg; the Compare POST
# terminated in its gap-04 lane); the read-relay leg is GET
# /api/v0/dependencies, which stays on the generic relay. The ACS leg
# moves the representative 288KiB IdP form, so it compares against the
# same bytes on the generic relay (BenchmarkD9SAMLRelayLargePost, the
# D9 lane's pinned same-bytes relay leg) — not against an empty-body
# POST. The SAML paths now terminate; every leg does exactly one
# server-side fetch of the same bytes, so the 1.50 bound stays a real
# pathology check either way.
#
# Usage: bash scripts/measure-saml-termination-latency.sh
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
  echo "leg ${leg}: term=${clone}ns relay=${relay}ns ratio=${ratio} (fail past 1.50)"
  if [[ "$over" == "1" ]]; then
    echo "FAIL: terminated leg ${leg} regresses past 1.50x of relay" >&2
    exit 1
  fi
}

term_metadata="$(bench 'BenchmarkSAMLTermMetadata/metadata' | median)"
term_login_bare="$(bench 'BenchmarkSAMLTermLogin/bare' | median)"
term_login_query="$(bench 'BenchmarkSAMLTermLogin/query' | median)"
term_acs="$(bench 'BenchmarkSAMLTermACS/acs' | median)"
relay_post="$(bench 'BenchmarkD8RelayWrite/post' | median)"
relay_get="$(bench 'BenchmarkD8RelayRead/get' | median)"
relay_large_post="$(bench 'BenchmarkD9SAMLRelayLargePost/post' | median)"

check_leg metadata "$term_metadata" "$relay_get"
check_leg login-bare "$term_login_bare" "$relay_get"
check_leg login-query "$term_login_query" "$relay_get"
check_leg login-query-vs-post "$term_login_query" "$relay_post"
check_leg acs-large-body "$term_acs" "$relay_large_post"

echo "saml-termination-latency: all legs within budget"
