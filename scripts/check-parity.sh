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
SRC_SHA="428790ab3ac0bff2eb28e50a26b3746a9b7dc8c5"
RAW="https://raw.githubusercontent.com/${SRC_REPO}/${SRC_SHA}"
# Per-family pins for seams that merged after SRC_SHA. Each family is
# fetched at the commit where its vendored files reached final form.
SRC_SHA_MAIN="ebaf4dfa0de672851bbec7a427e28fdb41344380"
SRC_SHA_SETUP="1b35b907ee237de64b0710d25e955fec43e9677b"

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

fetch_at() {
  local sha="$1" path="$2"
  curl -fsSL --retry 3 "https://raw.githubusercontent.com/${SRC_REPO}/${sha}/${path}" -o "${tmpdir}/fetched" || {
    echo "FAIL: fetch ${path} at ${sha}" >&2
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
  "7f9348271464c27ac27afa7201874dc5e06fd0f2ff0cb096dff9b02fedd76ee3"

check_file "go/internal/storage/postgres/db/read_store_contract_test.go" \
  "contracts/upstream/read_store_contract_test.go" \
  "b4d021da1c406870555ab4d414f1e7962ddf187d3c996e8012f10ddd70cd3166"

check_file "go/internal/query/admin/acknowledgement/admission_contract_test.go" \
  "contracts/upstream/terminal-failure_admission_contract_test.go" \
  "a76c88800bb0dc2f4e853632dfefde61fcc25648588ecb93036c296bb851ed47"

check_file "go/internal/query/admin/acknowledgement/replay_ledger_admission_contract_test.go" \
  "contracts/upstream/replay_ledger_admission_contract_test.go" \
  "0a7139031b54ca4005f71498dfb43784e1c75264aa758c8f313462c9a9ff1ba7"

check_file "go/internal/query/sign_in_policy_admission_contract_test.go" \
  "contracts/upstream/sign_in_policy_admission_contract_test.go" \
  "3ce68b7ea519acf581acb70bb42929d03deed0de5c99e6312329078c71c72842"

check_file_at() {
  local sha="$1" src_path="$2" dest_rel="$3" want_hash="$4"
  local fetched
  fetched="$(fetch_at "$sha" "$src_path")"
  local got_hash
  got_hash="$(sha256sum "$fetched" | cut -d' ' -f1)"
  if [[ "$got_hash" != "$want_hash" ]]; then
    echo "FAIL: ${src_path} at ${sha} hash ${got_hash} != recorded ${want_hash}" >&2
    exit 1
  fi
  if ! cmp -s "$fetched" "${REPO_ROOT}/${dest_rel}"; then
    echo "FAIL: ${dest_rel} differs from ${src_path} at ${sha}" >&2
    exit 1
  fi
  echo "OK: ${dest_rel} byte-identical to ${src_path} at ${sha}"
}

check_file_at "$SRC_SHA_MAIN" "go/internal/query/admin/retirement/admission_contract_test.go" \
  "contracts/upstream/retirement_admission_contract_test.go" \
  "8ade12f13bd24786ee348c27807b223e24a685abe7d114b46abfeff1ebe926dd"

check_file_at "$SRC_SHA_MAIN" "go/internal/query/admin/identity/admission_contract_test.go" \
  "contracts/upstream/admin_identity_admission_contract_test.go" \
  "1dcf3c4cb97170cbd0c3a33d65539e3a1472ff66381e2a9762e9a60c96826559"

check_file_at "$SRC_SHA_SETUP" "go/internal/query/setup_admission_contract_test.go" \
  "contracts/upstream/setup_admission_contract_test.go" \
  "b5dc9640b1683fd0cd259343483201ecd3c0c5a5d68262205fa65c3cb827a0ea"

# Wire-shape excerpt: every vendored line must appear verbatim in at
# least one of the pinned source files (whole-line match,
# order-insensitive).
check_excerpt() {
  local dest_rel="$1"
  shift
  local excerpt_ok=1
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    local found=0
    for src_path in "$@"; do
      local excerpt_src
      excerpt_src="$(fetch "$src_path")"
      if grep -qFx -- "$line" "$excerpt_src"; then
        found=1
        break
      fi
    done
    if [[ "$found" != "1" ]]; then
      echo "FAIL: excerpt line missing from pinned sources: ${line}" >&2
      excerpt_ok=0
    fi
  done < "${REPO_ROOT}/${dest_rel}"
  if [[ "$excerpt_ok" != "1" ]]; then
    exit 1
  fi
  echo "OK: ${dest_rel} lines all present in pinned sources"
}

check_excerpt "contracts/upstream/suppression-wire-shapes.txt" \
  "go/internal/query/supply/chain/suppression_mutation.go"

check_excerpt "contracts/upstream/terminal-failure-wire-shapes.txt" \
  "go/internal/query/admin/acknowledgement/admission.go" \
  "go/internal/query/admin/acknowledgement/handler.go"

check_excerpt "contracts/upstream/replay-ledger-wire-shapes.txt" \
  "go/internal/query/admin/acknowledgement/admission.go" \
  "go/internal/query/admin/acknowledgement/handler.go"

check_excerpt "contracts/upstream/sign-in-wire-shapes.txt" \
  "go/internal/query/sign_in_policy_types.go"

check_excerpt_at() {
  local sha="$1" dest_rel="$2"
  shift 2
  local excerpt_ok=1
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    local found=0
    for src_path in "$@"; do
      local excerpt_src
      excerpt_src="$(fetch_at "$sha" "$src_path")"
      if grep -qFx -- "$line" "$excerpt_src"; then
        found=1
        break
      fi
    done
    if [[ "$found" != "1" ]]; then
      echo "FAIL: excerpt line missing from pinned sources at ${sha}: ${line}" >&2
      excerpt_ok=0
    fi
  done < "${REPO_ROOT}/${dest_rel}"
  if [[ "$excerpt_ok" != "1" ]]; then
    exit 1
  fi
  echo "OK: ${dest_rel} lines all present in pinned sources at ${sha}"
}

check_excerpt_at "$SRC_SHA_MAIN" "contracts/upstream/retirement-wire-shapes.txt" \
  "go/internal/query/admin/retirement/admission.go" \
  "go/internal/query/admin/retirement/handler.go"

check_excerpt_at "$SRC_SHA_MAIN" "contracts/upstream/admin-identity-wire-shapes.txt" \
  "go/internal/query/admin/identity/admission.go" \
  "go/internal/query/admin/identity/types.go"

check_excerpt_at "$SRC_SHA_SETUP" "contracts/upstream/setup-wire-shapes.txt" \
  "go/internal/query/setup_types.go"

# Version pins referenced by the contract records.
grep -qF 'const CoreReadContractsVersion = "v0"' "${REPO_ROOT}/contracts/upstream/api_boundary.go"
grep -qF 'SuppressionAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/suppression-wire-shapes.txt"
grep -qF 'TerminalFailureAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/terminal-failure-wire-shapes.txt"
grep -qF 'ReplayLedgerAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/replay-ledger-wire-shapes.txt"
grep -qF 'SignInPolicyAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/sign-in-wire-shapes.txt"
grep -qF 'RetirementAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/retirement-wire-shapes.txt"
grep -qF 'AdminIdentityAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/admin-identity-wire-shapes.txt"
grep -qF 'SetupAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/setup-wire-shapes.txt"
echo "OK: version pins v0 present"

echo "parity: all checks passed"
