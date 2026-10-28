#!/usr/bin/env bash
# Parity gate for the eshu-api seed (Refs eshu-hq/eshu#8872).
#
# Re-fetches each pinned source file from eshu-hq/eshu at the provenance
# commit and proves the vendored copies are byte-exact: full-file sha256
# for the verbatim files, line-presence for the wire-shape excerpt.
# Fails closed on any fetch error, hash mismatch, or missing line.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC_REPO="eshu-hq/eshu"
SRC_SHA="e4c385f493494ed9d6c7f192b04ce0255361f621"
RAW="https://raw.githubusercontent.com/${SRC_REPO}/${SRC_SHA}"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

fetch() {
  local path="$1"
  curl -fsSL --retry 3 "${RAW}/${path}" -o "${tmpdir}/fetched" || {
    echo "FAIL: fetch ${path} at ${SRC_SHA}" >&2
    exit 1
  }
  printf '%s' "${tmpdir}/fetched"
}

check_file() {
  local src_path="$1" dest_rel="$2" want_hash="$3"
  local fetched
  fetched="$(fetch "$src_path")"
  local got_hash
  got_hash="$(sha256sum "$fetched" | cut -d' ' -f1)"
  if [[ "$got_hash" != "$want_hash" ]]; then
    echo "FAIL: ${src_path} hash ${got_hash} != recorded ${want_hash}" >&2
    exit 1
  fi
  if ! cmp -s "$fetched" "${REPO_ROOT}/${dest_rel}"; then
    echo "FAIL: ${dest_rel} differs from ${src_path} at ${SRC_SHA}" >&2
    exit 1
  fi
  echo "OK: ${dest_rel} byte-identical to ${src_path}"
}

check_file "go/internal/query/querycontract/api_boundary.go" \
  "contracts/upstream/api_boundary.go" \
  "c5f3ab34bc6e8d43c119b9aa7028ba298fb7f15cc68ffd8386e63ad9c140d9a1"

check_file "go/internal/query/querycontract/api_boundary_test.go" \
  "contracts/upstream/api_boundary_test.go" \
  "346fd5fe3a40257812108e7d4fb72c3e683a28f0412e121781e8bcb9349a92b8"

check_file "go/internal/query/supply/chain/suppression_admission_contract_test.go" \
  "contracts/upstream/suppression_admission_contract_test.go" \
  "f9e08e5ee8f69f6ffb97a14be6995c7f9497e67f8317c5338166a83246318552"

check_file "go/internal/storage/postgres/db/read_store_contract_test.go" \
  "contracts/upstream/read_store_contract_test.go" \
  "b4d021da1c406870555ab4d414f1e7962ddf187d3c996e8012f10ddd70cd3166"

# Wire-shape excerpt: every vendored line must appear verbatim in the
# pinned suppression_mutation.go (whole-line match, order-insensitive).
excerpt_src="$(fetch "go/internal/query/supply/chain/suppression_mutation.go")"
excerpt_ok=1
while IFS= read -r line; do
  [[ -z "$line" ]] && continue
  if ! grep -qFx -- "$line" "$excerpt_src"; then
    echo "FAIL: excerpt line missing from pinned source: ${line}" >&2
    excerpt_ok=0
  fi
done < "${REPO_ROOT}/contracts/upstream/suppression-wire-shapes.txt"
if [[ "$excerpt_ok" != "1" ]]; then
  exit 1
fi
echo "OK: contracts/upstream/suppression-wire-shapes.txt lines all present in pinned source"

# Version pins referenced by the contract records.
grep -qF 'const CoreReadContractsVersion = "v0"' "${REPO_ROOT}/contracts/upstream/api_boundary.go"
grep -qF 'SuppressionAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/suppression-wire-shapes.txt"
echo "OK: version pins v0/v0 present"

echo "parity: all checks passed"
