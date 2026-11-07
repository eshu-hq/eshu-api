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
SRC_SHA_SETUP="fd326f35c9fb497eec0abefad197a5d4b137b823"
SRC_SHA_PROVIDERCONFIG="99381691b1ac49254e813e5a013cfb654d432491"
SRC_SHA_AUTHLOGIN="1e03478614fc54882b9616d37ea9103ea5ada6f4"
SRC_SHA_SAMLLOGIN="aac271ad78e328462dabc7518d5f7e5739bdc651"
SRC_SHA_RECOVERY="7c60583996d9a3ccc46288f40a38f614a4f28ea8"
SRC_SHA_D1="2c89a9d8b0a5efa1c4fbdd1d3e24de00012d1923"
SRC_SHA_D0="ec1643d67f"
SRC_SHA_WAVE="17feac86e089a6da208fe6c93b5d89b8fb7ef3cc"
SRC_SHA_D6="6b66e6b23efe226970b4571c955aed1dc42f987a"
SRC_SHA_D7="19de7346d11a184bf88d8f307b5d7a9ced0a4657"
SRC_SHA_D8="91f2df28dbe9e64ecf5812c5f89a76475a304d84"

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

# Native spec (Decision 5(c)): the sign-in-policy OpenAPI fragment is a
# byte-identical port of the pinned core file, gated here like the
# upstream evidence above.
check_file "go/internal/query/openapi/paths/auth/sign_in_policy.go" \
  "openapi/paths/auth/sign_in_policy.go" \
  "8e158762038a0d5e1719b1b75e865b80962be88a92e69872453fc300e0174cf3"

# Native spec (Decision 5(c)): the terminal-failure OpenAPI fragment is a
# byte-identical port of the pinned core file, gated here like the
# upstream evidence above.
check_file "go/internal/query/openapi/paths/status/collector_terminal_failure_acknowledgement.go" \
  "openapi/paths/status/collector_terminal_failure_acknowledgement.go" \
  "238401aab7e91208d466cd897beb7257d2be8f845ee4cc5025e35b15746e0121"

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

check_fragment_subset_at() {
  local sha="$1" src_path="$2" dest_rel="$3" want_hash="$4"
  local fetched
  fetched="$(fetch_at "$sha" "$src_path")"
  local got_hash
  got_hash="$(sha256sum "$fetched" | cut -d' ' -f1)"
  if [[ "$got_hash" != "$want_hash" ]]; then
    echo "FAIL: ${src_path} at ${sha} hash ${got_hash} != recorded ${want_hash}" >&2
    exit 1
  fi
  # The payload is the const string body: every line after the
  # `const NAME = `` opener up to the closing backtick. Each payload
  # line must appear verbatim in the pinned core file.
  local in_payload=0
  while IFS= read -r line; do
    if [[ "$line" == 'const AdminProviderConfigMutations = `' ]]; then
      in_payload=1
      continue
    fi
    if [[ "$in_payload" != "1" ]]; then
      continue
    fi
    if [[ "$line" == '`' ]]; then
      break
    fi
    [[ -z "$line" ]] && continue
    if ! grep -qFx -- "$line" "$fetched"; then
      echo "FAIL: fragment line missing from ${src_path} at ${sha}: ${line}" >&2
      exit 1
    fi
  done < "${REPO_ROOT}/${dest_rel}"
  if [[ "$in_payload" != "1" ]]; then
    echo "FAIL: ${dest_rel} carries no AdminProviderConfigMutations payload" >&2
    exit 1
  fi
  echo "OK: ${dest_rel} lines all verbatim in ${src_path} at ${sha}"
}

check_fragment_subset_named_at() {
  local sha="$1" src_path="$2" dest_rel="$3" want_hash="$4" const_name="$5"
  local fetched
  fetched="$(fetch_at "$sha" "$src_path")"
  local got_hash
  got_hash="$(sha256sum "$fetched" | cut -d' ' -f1)"
  if [[ "$got_hash" != "$want_hash" ]]; then
    echo "FAIL: ${src_path} at ${sha} hash ${got_hash} != recorded ${want_hash}" >&2
    exit 1
  fi
  # The payload is the const string body: every line after the
  # `const NAME = `` opener up to the closing backtick. Each payload
  # line must appear verbatim in the pinned core file. The const name
  # is a parameter (unlike check_fragment_subset_at above) so later
  # lanes reuse this gate without copying it.
  local in_payload=0
  while IFS= read -r line; do
    if [[ "$line" == "const ${const_name} = \`" ]]; then
      in_payload=1
      continue
    fi
    if [[ "$in_payload" != "1" ]]; then
      continue
    fi
    if [[ "$line" == '`' ]]; then
      break
    fi
    [[ -z "$line" ]] && continue
    if ! grep -qFx -- "$line" "$fetched"; then
      echo "FAIL: fragment line missing from ${src_path} at ${sha}: ${line}" >&2
      exit 1
    fi
  done < "${REPO_ROOT}/${dest_rel}"
  if [[ "$in_payload" != "1" ]]; then
    echo "FAIL: ${dest_rel} carries no ${const_name} payload" >&2
    exit 1
  fi
  echo "OK: ${dest_rel} lines all verbatim in ${src_path} at ${sha}"
}

check_recovery_schemas_at() {
  local sha="$1"
  local admin_comp="${tmpdir}/admin_components.go"
  local recovery_comp="${tmpdir}/recovery_components.go"
  curl -fsSL --retry 3 "https://raw.githubusercontent.com/${SRC_REPO}/${sha}/go/internal/query/openapi/components.go" -o "$admin_comp" || {
    echo "FAIL: fetch go/internal/query/openapi/components.go at ${sha}" >&2
    exit 1
  }
  curl -fsSL --retry 3 "https://raw.githubusercontent.com/${SRC_REPO}/${sha}/go/internal/query/openapi/components_recovery.go" -o "$recovery_comp" || {
    echo "FAIL: fetch go/internal/query/openapi/components_recovery.go at ${sha}" >&2
    exit 1
  }
  # The three lane schemas form one contiguous block in
  # openapi/components.json: from the SkippedScopesReport opener through
  # the third six-space closer (SkippedScopesReport, then
  # DeltaActiveScopesReport, then ReindexRequestsWrittenReport). The
  # last closer carries no trailing comma — the lane block is the last
  # entry of the schemas object — so the match tolerates it.
  local block
  block="$(awk '/^      "SkippedScopesReport": \{$/{flag=1} flag{print} flag && /^      \},?$/ && ++closes==3{exit}' "${REPO_ROOT}/openapi/components.json")"
  if [[ -z "$block" ]]; then
    echo "FAIL: no recovery schema block in openapi/components.json" >&2
    exit 1
  fi
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    if grep -qFx -- "$line" "$admin_comp" || grep -qFx -- "$line" "$recovery_comp"; then
      continue
    fi
    # The DeltaActiveScopesReport opener rides the Go const line in
    # core, so no whole line matches; the substring must be present and
    # the served-spec LocalRefsResolve test independently requires it.
    if [[ "$line" == '      "DeltaActiveScopesReport": {' ]] && grep -qF -- '"DeltaActiveScopesReport": {' "$recovery_comp"; then
      continue
    fi
    # The lane block is the last entry of the schemas object, so its
    # final closer drops the comma core carries (more entries follow
    # there); the comma form must be present.
    if [[ "$line" == '      }' ]] && grep -qFx -- '      },' "$recovery_comp"; then
      continue
    fi
    echo "FAIL: recovery schema line missing from pinned component sources at ${sha}: ${line}" >&2
    exit 1
  done <<< "$block"
  echo "OK: recovery schema block lines all verbatim in pinned component sources at ${sha}"
}

check_file_at "$SRC_SHA_MAIN" "go/internal/query/admin/retirement/admission_contract_test.go" \
  "contracts/upstream/retirement_admission_contract_test.go" \
  "8ade12f13bd24786ee348c27807b223e24a685abe7d114b46abfeff1ebe926dd"

# Native spec (Decision 5(c)): the retirement OpenAPI fragment is a
# byte-identical port of the pinned core file, gated here like the
# upstream evidence above.
check_file_at "$SRC_SHA_MAIN" "go/internal/query/openapi/paths/status/repository_retirement.go" \
  "openapi/paths/status/repository_retirement.go" \
  "d576e641f9bf9e95ea3f5e81d73b6b185ee88bb54fe9c64301524b7f801c3fef"

# Native spec (Decision 5(c)): the admin-identity mutation OpenAPI
# fragment is a byte-identical port of the pinned core file, gated here
# like the upstream evidence above.
check_file_at "$SRC_SHA_MAIN" "go/internal/query/openapi/paths/auth/admin_mutations.go" \
  "openapi/paths/auth/admin_mutations.go" \
  "c36f06548db46242940cd7f2661f22f8148706e6e554df61133031e84878f886"

# Native spec (Decision 5(c)): the admin-identity reads OpenAPI
# fragment is a byte-identical port of the pinned core file, gated here
# like the upstream evidence above. Reads stay relay (no native routes
# added — fragment only); the grant and create POSTs share path keys
# with the family GETs, so they live in this reads fragment rather than
# the mutations fragment.
check_file_at "$SRC_SHA_MAIN" "go/internal/query/openapi/paths/auth/admin_reads.go" \
  "openapi/paths/auth/admin_reads.go" \
  "ce77d72fdd6b448b8a42e9b15981fbdc2053beeadbecd46ba61dbd427365da6e"

check_file_at "$SRC_SHA_MAIN" "go/internal/query/admin/identity/admission_contract_test.go" \
  "contracts/upstream/admin_identity_admission_contract_test.go" \
  "1dcf3c4cb97170cbd0c3a33d65539e3a1472ff66381e2a9762e9a60c96826559"

check_file_at "$SRC_SHA_SETUP" "go/internal/query/setup_admission_contract_test.go" \
  "contracts/upstream/setup_admission_contract_test.go" \
  "b5dc9640b1683fd0cd259343483201ecd3c0c5a5d68262205fa65c3cb827a0ea"

check_file_at "$SRC_SHA_PROVIDERCONFIG" "go/internal/query/admin/provider/config/mutation_admission_contract_test.go" \
  "contracts/upstream/mutation_admission_contract_test.go" \
  "15bd3551a50b503919a065da7fa8e077a0ac7179b3f3a9e8160673592f4d0e3b"

check_file_at "$SRC_SHA_PROVIDERCONFIG" "go/internal/query/admin/provider/config/read_admission_contract_test.go" \
  "contracts/upstream/read_admission_contract_test.go" \
  "51d80f84961ea7b8e25e51c69b5c406b38f00ed77edfafd31c419450bfc4886d"

# Native spec (Decision 5(c)): the provider-config mutation OpenAPI
# fragment is a line-verbatim subset of the pinned core file. Core ships
# reads and mutations in one file, so no whole-file identity port can
# serve mutations only; the gate pins the core file's sha256 and proves
# every payload line of the dest fragment (the const string body, which
# is the only JSON the assembly consumes) appears verbatim in the pinned
# core file. The create/update POSTs share path keys with the family
# GETs, so per the core OpenAPI convention they stay in the unported
# reads fragment; the reads-split follow-up ports them.
check_fragment_subset_at "$SRC_SHA_PROVIDERCONFIG" "go/internal/query/openapi/paths/auth/admin_provider_configs.go" \
  "openapi/paths/auth/admin_provider_config_mutations.go" \
  "afa45de81720f466b8b9f66949605564be21e9efa8ddb8c62e71600b127824ef"

check_file_at "$SRC_SHA_AUTHLOGIN" "go/internal/query/oidc_login_admission_contract_test.go" \
  "contracts/upstream/oidc_login_admission_contract_test.go" \
  "03342f7ad0d28aef72583ce21a7747fe360fe755367fef40660e58361f19d2c0"

check_file_at "$SRC_SHA_AUTHLOGIN" "go/internal/query/github_login_admission_contract_test.go" \
  "contracts/upstream/github_login_admission_contract_test.go" \
  "f6a7fc9920176c76c8cf6e5ec223e881323564405b5931770d43db9f7256889c"

check_file_at "$SRC_SHA_SAMLLOGIN" "go/internal/query/saml_login_admission_contract_test.go" \
  "contracts/upstream/saml_login_admission_contract_test.go" \
  "bc04787bd208bf07428f4a9bc4660946342c7e4f3b60121202cb7f0a070556d0"

check_file_at "$SRC_SHA_RECOVERY" "go/internal/query/admin/recovery_admission_contract_test.go" \
  "contracts/upstream/recovery_admission_contract_test.go" \
  "50dd78f3dc9bde8fdc67d7f818e0b2fdfdeb22fa7ce62d8650c361345826af24"

# D1 read-admission pins (eshu-hq/eshu#10259, unwired seams): the four
# frozen method-set tests, byte-identical to the pinned core commit.
check_file_at "$SRC_SHA_D1" "go/internal/query/admin/read_admission_contract_test.go" \
  "contracts/upstream/admin_read_admission_contract_test.go" \
  "51148149f97cd6613f6d7349d4f6dbb02ca55de9bf00e23cfcf410491cfe6784"

check_file_at "$SRC_SHA_D1" "go/internal/query/admin/identity/read_admission_contract_test.go" \
  "contracts/upstream/admin_identity_read_admission_contract_test.go" \
  "50e02d231061f28f261670bddeacec828e0e01f43269ab4cf9ed6045c217011f"

check_file_at "$SRC_SHA_D1" "go/internal/query/admin/identity/governance_audit_read_admission_contract_test.go" \
  "contracts/upstream/governance_audit_read_admission_contract_test.go" \
  "b6b033cfdbdaf9cb788b1e7354e199caeda5b1604c7230e0b58caf97b3b2aef5"

check_file_at "$SRC_SHA_D1" "go/internal/query/admin/acknowledgement/terminal_failure_read_admission_contract_test.go" \
  "contracts/upstream/terminal_failure_read_admission_contract_test.go" \
  "c0e86ef4e90ff24e6756f27bd870675782baeeeb9b1c37f9e0551e0cb035f51a"

# D6 read-admission pins (eshu-hq/eshu#10303, unwired seams): the two
# frozen method-set tests, byte-identical to the pinned core commit.
check_file_at "$SRC_SHA_D6" "go/internal/status/read_admission_contract_test.go" \
  "contracts/upstream/status_read_admission_contract_test.go" \
  "83f8a4a15c24aa93c3d82dc14da1880470be58301aeac10f4944f28f57a90233"

check_file_at "$SRC_SHA_D6" "go/internal/query/supply/chain/supply_read_admission_contract_test.go" \
  "contracts/upstream/supply_read_admission_contract_test.go" \
  "8132db71af6058478332913bfd260190dfd98e8a09b9230a742b4d2ecc2a7367"

# D7 admin-four pins (eshu-hq/eshu#10312, unwired seams): the four
# frozen method-set tests, byte-identical to the pinned core commit.
check_file_at "$SRC_SHA_D7" "go/internal/query/admin/backfill_admission_contract_test.go" \
  "contracts/upstream/backfill_admission_contract_test.go" \
  "50fb9462a3aa2d4869bf43cab7e8af69a92dc5a7a9625339fd6b86be34b5efbe"

check_file_at "$SRC_SHA_D7" "go/internal/query/admin/dead_letter_admission_contract_test.go" \
  "contracts/upstream/dead_letter_admission_contract_test.go" \
  "9c583e7ce8c06384b8c6642f1fea0c1d4581b40d84ca7dae3f94f2f192a56ddb"

check_file_at "$SRC_SHA_D7" "go/internal/query/admin/reindex_admission_contract_test.go" \
  "contracts/upstream/reindex_admission_contract_test.go" \
  "061092ea83a6447af59298e3df5d49ff7878eff0df72bc43e3b7c8fafef44767"

check_file_at "$SRC_SHA_D7" "go/internal/query/admin/skip_admission_contract_test.go" \
  "contracts/upstream/skip_admission_contract_test.go" \
  "0486124e708486faaa0dafd9e00fb387a03f228ea8676e9c82ca63c60d67eaae"

# Native spec (Decision 5(c)): the admin-recovery OpenAPI fragment is a
# line-verbatim subset of the pinned core file. Core ships the lane
# entries and the relay-only routes in one file, so no whole-file
# identity port can serve this lane only; the gate pins the core file's
# sha256 and proves every payload line of the dest fragment (the const
# string body, which is the only JSON the assembly consumes) appears
# verbatim in the pinned core file. The relay-only four keep their own
# path keys there and stay out of this fragment, so no served key
# duplicates a relayed route.
check_fragment_subset_named_at "$SRC_SHA_RECOVERY" "go/internal/query/openapi/paths/status/admin.go" \
  "openapi/paths/status/admin_recovery.go" \
  "26c7669744ebb6b29a18bb9d95c7186e24785421afb8d733d9d9a3a246a78eb9" \
  "AdminRecovery"

# Native spec (Decision 5(c)): the admin-four OpenAPI fragment is a
# line-verbatim subset of the pinned core file at the D7 merge. The D7
# merge leaves admin.go byte-identical to the recovery pin (same
# sha256), so the gate pins that same whole-file hash at the D7 commit
# and proves every payload line of the dest fragment appears verbatim
# in it. The four lane path keys (backfill, dead-letter, reindex,
# skip) are unique to this lane and separable from the C-c2, D3, and
# relay keys sharing that file, so no served key duplicates another
# lane's route.
check_fragment_subset_named_at "$SRC_SHA_D7" "go/internal/query/openapi/paths/status/admin.go" \
  "openapi/paths/status/admin_d7.go" \
  "26c7669744ebb6b29a18bb9d95c7186e24785421afb8d733d9d9a3a246a78eb9" \
  "AdminD7"

# D8 session-resolver pins (satellite D8 wave, arbiter
# APPROVED-WITH-CONDITIONS): the four v0 admission pins
# (SessionResolver, BrowserSession, LocalIdentity, APIToken), unwired
# seams, recorded byte-identical satellite-side first (D7 #57
# precedent).
check_file_at "$SRC_SHA_D8" "go/internal/query/session_admission.go" \
  "contracts/upstream/session_admission.go" \
  "11d1dd3415a7a925a181016eb8aefbb4dedccbd542a2798c729972c06062676d"

check_file_at "$SRC_SHA_D8" "go/internal/query/session_admission_contract_test.go" \
  "contracts/upstream/session_admission_contract_test.go" \
  "209a3c2dac30e2060dc8012341f3fa79d2e47ed61f2cc4048645d04a2496b341"

# D8 session-resolver wave (Decision 5(c)): the browser-session
# lifecycle, session-list, profile, and local-identity entries are
# line-verbatim subsets of the pinned core routes.go (routes.go is
# byte-identical at the D2 wave pin and the D8 pin, hash 709fe6b3…,
# so the D8 gates below and the D2/D3 gates above pin the same bytes
# from their own lane pins). The invitations POST merges into the D3
# fragment key; every lane's path keys stay unique.
check_fragment_subset_named_at "$SRC_SHA_D8" "go/internal/query/openapi/paths/auth/routes.go" \
  "openapi/paths/auth/browser_sessions.go" \
  "709fe6b3e96967fa615bff59a8c02cb8c1c5fbc6a05744bf57835170a5ad633a" \
  "BrowserSessions"

check_fragment_subset_named_at "$SRC_SHA_D8" "go/internal/query/openapi/paths/auth/routes.go" \
  "openapi/paths/auth/session_list.go" \
  "709fe6b3e96967fa615bff59a8c02cb8c1c5fbc6a05744bf57835170a5ad633a" \
  "SessionList"

check_fragment_subset_named_at "$SRC_SHA_D8" "go/internal/query/openapi/paths/auth/routes.go" \
  "openapi/paths/auth/auth_profile.go" \
  "709fe6b3e96967fa615bff59a8c02cb8c1c5fbc6a05744bf57835170a5ad633a" \
  "AuthProfile"

check_fragment_subset_named_at "$SRC_SHA_D8" "go/internal/query/openapi/paths/auth/routes.go" \
  "openapi/paths/auth/local_identity.go" \
  "709fe6b3e96967fa615bff59a8c02cb8c1c5fbc6a05744bf57835170a5ad633a" \
  "LocalIdentity"

check_fragment_subset_named_at "$SRC_SHA_D8" "go/internal/query/openapi/paths/auth/routes.go" \
  "openapi/paths/auth/admin_invitations.go" \
  "709fe6b3e96967fa615bff59a8c02cb8c1c5fbc6a05744bf57835170a5ad633a" \
  "AdminInvitations"

# D8 session-resolver wave (Decision 5(c)): the API-token and TOTP
# entries are whole-file byte-identical ports of the pinned core
# files (each file carries only its lane's path keys).
check_file_at "$SRC_SHA_D8" "go/internal/query/openapi/paths/auth/tokens.go" \
  "openapi/paths/auth/api_tokens.go" \
  "465aae5df7bc4cc938c4ecbf5800ed704c20447111a9c115f6c07c87393aec59"

check_file_at "$SRC_SHA_D8" "go/internal/query/openapi/paths/auth/totp.go" \
  "openapi/paths/auth/totp.go" \
  "702ab2b220717202910c39a792fb7b9da465d75e38101dc915c472d9322a7238"

# D8 session-resolver wave (Decision 5(c)): the 11 schemas the lane
# entries reference, ported line-verbatim into the shared components
# block — nine LocalIdentity schemas from components_local_identity.go
# plus BrowserSessionAuth and BrowserSessionResponse from
# components_workload_session.go.
# Native spec (Decision 5(c)): the suppression OpenAPI fragment is a
# line-verbatim subset port of the pinned core file. Core ships the
# payload under the unexported const suppressionMutation, so the
# dest-owned exported wrapper differs by the const line only; the gate
# pins the core file's sha256 and proves every payload line of the dest
# fragment (the const string body, which is the only JSON the assembly
# consumes) appears verbatim in the pinned core file. The file carries
# the lane's single path key, so no served key duplicates a relayed
# route; the fragment only closes the Decision 5(c) gap the C-b ruling
# left open for the pilot family.
check_fragment_subset_named_at "$SRC_SHA_D0" "go/internal/query/openapi/paths/supply/chain/suppression_mutation.go" \
  "openapi/paths/supply/chain/suppression_mutation.go" \
  "5eb2b02ad1bdf49118d758853dbb51afe26ec87604f59d1ca2399ee571e95172" \
  "SuppressionMutation"

# Recovery schemas (Decision 5(c)): the three lane schemas form one
# contiguous block in openapi/components.json, from the SkippedScopesReport
# opener through the ReindexRequestsWrittenReport closer. Every line of
# that block must appear verbatim in one of the two pinned component
# sources — except the DeltaActiveScopesReport opener, which rides the
# Go const line in core (verified as a substring instead), and which
# the served-spec LocalRefsResolve test independently requires.
check_recovery_schemas_at "$SRC_SHA_RECOVERY"

# D2 Leaf B Native-now wave (Decision 5(c)): the wave fragments are
# byte-identical ports (whole-file) or line-verbatim subsets of the
# pinned core files at SRC_SHA_WAVE — except the provider-config reads
# subset, which reuses the provider-config lane pin (the file is
# byte-identical at both pins, hash afa45de8...). The doc-UI shells are
# line-verbatim subsets of the pinned core openapi/spec.go.
check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/catalog/capabilities.go" \
  "openapi/paths/catalog/capabilities.go" \
  "e29adb2cc78925212cc8e4c2882b063f2bcf15ff867b03a72a2febd2e8937229"

check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/cli/score.go" \
  "openapi/paths/cli/score.go" \
  "2cf44ef41567879b6ffc7f3cdc8468b51beaaf2e3ffcf528161c8fd65f954570"

check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/cli/render.go" \
  "openapi/paths/cli/render.go" \
  "6cdc25b8204c86417dd8659f2416561e2032a5792424659aa50516563e7bbba2"

check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/cli/capture.go" \
  "openapi/paths/cli/capture.go" \
  "9760b71696292fb8d83b79580d4e65bf9cc1dde0537e63e9f51c173f199b5aab"

check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/cli/validate.go" \
  "openapi/paths/cli/validate.go" \
  "159f8c031d1dbf8579a951a452648d3f7a52ec753ca538e8121b7ff4ec339b7f"

check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/cli/compose.go" \
  "openapi/paths/cli/compose.go" \
  "ef7a8b98c253956ee5bb8575e73d9af05373d3cf899e59221eed5feb370421c6"

check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/search/ask.go" \
  "openapi/paths/search/ask.go" \
  "6d1640ad03ddfa351a4df611d6faee451b3403dc39d02dee893d7115a8eb5740"

check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/service/intelligence_report.go" \
  "openapi/paths/service/intelligence_report.go" \
  "d7f43593264272966a11be43855a83595dc863545790e2f08711aa4a4472a747"

check_fragment_subset_named_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/auth/routes.go" \
  "openapi/paths/auth/auth_providers.go" \
  "709fe6b3e96967fa615bff59a8c02cb8c1c5fbc6a05744bf57835170a5ad633a" \
  "AuthProviders"

check_fragment_subset_named_at "$SRC_SHA_PROVIDERCONFIG" "go/internal/query/openapi/paths/auth/admin_provider_configs.go" \
  "openapi/paths/auth/admin_provider_config_reads.go" \
  "afa45de81720f466b8b9f66949605564be21e9efa8ddb8c62e71600b127824ef" \
  "AdminProviderConfigReads"

check_fragment_subset_named_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/status/routes.go" \
  "openapi/paths/status/api_router_openapi.go" \
  "24a57b35c675cb33ad5011343a0aeb68d49e9d05d7645f397de2d88e8e5afa61" \
  "OpenAPISpec"

# D3 admin/identity/recovery reads (Decision 5(c)): the six admin
# query POSTs are a line-verbatim subset of the pinned core admin.go
# (same file and pin as the recovery lane — admin.go is byte-identical
# at both pins, hash 26c76697...), and the invitations list GET is a
# line-verbatim subset of the pinned core routes.go (same file and pin
# as the D2 auth-providers subset). The 3 unpinned Admin GETs, the
# relay-only mutations, and the invitations create POST keep their own
# path keys there and stay out of these fragments, so no served key
# duplicates a relayed route — except the invitations key, which the
# served document carries get-only until D8 ports the POST.
check_fragment_subset_named_at "$SRC_SHA_RECOVERY" "go/internal/query/openapi/paths/status/admin.go" \
  "openapi/paths/status/admin_reads.go" \
  "26c7669744ebb6b29a18bb9d95c7186e24785421afb8d733d9d9a3a246a78eb9" \
  "AdminReads"

check_fragment_subset_named_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/auth/routes.go" \
  "openapi/paths/auth/admin_invitations.go" \
  "709fe6b3e96967fa615bff59a8c02cb8c1c5fbc6a05744bf57835170a5ad633a" \
  "AdminInvitations"

# D4 Native-next wave (Decision 5(c)): the CICD, Documentation, Infra,
# PackageRegistry, Repositories, Evidence, InvestigationWorkflows, and
# Playbooks fragments are byte-identical ports (whole-file) or
# line-verbatim subsets of the pinned core files at SRC_SHA_WAVE (every
# file below is byte-identical at the D2 wave pin and at origin/main,
# so no new source pin is minted — see docs/provenance.md).
check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/cicd/routes.go" \
  "openapi/paths/cicd/routes.go" \
  "ddf2537b643884319ef98554f883a37a58f88bcab026ad94c32b34b747e07167"

check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/cicd/run_correlation_aggregate.go" \
  "openapi/paths/cicd/run_correlation_aggregate.go" \
  "f69d8339a3fa2a926ec5173f0655ac1bf1f3411b13438e310cff3f221bdb409d"

check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/evidence/documentation.go" \
  "openapi/paths/evidence/documentation.go" \
  "3fa8a202c0ac4a3935eb1b98c097c535e0ba95d17f0f07a2491815a458183635"

check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/evidence/documentation_finding_aggregate.go" \
  "openapi/paths/evidence/documentation_finding_aggregate.go" \
  "8c7eaa6fb39036ea12a65bd10ee092e534d620651c9b33a1fcbb25fae4db9b72"

check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/evidence/bundle.go" \
  "openapi/paths/evidence/bundle.go" \
  "f62e158efa2734993c1fb4c3ecd8d0a5f24e36343937bba3048fa29b9b4b48a2"

check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/evidence/investigation_workflows.go" \
  "openapi/paths/evidence/investigation_workflows.go" \
  "347f93d71ca8e54a7cb0350829261ffcb4661afdace73d69352912168f886054"

check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/cloud/routes.go" \
  "openapi/paths/cloud/routes.go" \
  "393f69d6bf1f71a5cd271c3e62fabb44ab22bf2a1ab3010243fe90ee30d478db"

check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/infrastructure/routes.go" \
  "openapi/paths/infrastructure/routes.go" \
  "6ce1fdca8609a6c4d6c56c7adf3ecfa6d31387ad64077c4780c62d245d03e136"

check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/infrastructure/resource_aggregate.go" \
  "openapi/paths/infrastructure/resource_aggregate.go" \
  "52caf0997f965428af702d0ebfbc99f3ada3007d3979e993f74c06ad01a64765"

check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/repository/package_registry.go" \
  "openapi/paths/repository/package_registry.go" \
  "b47f0506bdf450443d689bfeb815f4a72b4d2ba367e2686db6d5918c5472517c"

check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/repository/package_registry_aggregate.go" \
  "openapi/paths/repository/package_registry_aggregate.go" \
  "9cc3627c261b109d31e83e12b860c988671078d74affa88cf045e8429688d8eb"

check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/repository/branches.go" \
  "openapi/paths/repository/branches.go" \
  "1b92ee9e992dbf3cbca68983687ce31dde638193b16de965a3085d5222fd9d67"

check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/repository/stats_and_coverage.go" \
  "openapi/paths/repository/stats_and_coverage.go" \
  "2823f88f0659e77fa347ff72eb9f1bd3147343ad022efb737ab1201eb0bc293a"

check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/repository/freshness.go" \
  "openapi/paths/repository/freshness.go" \
  "9b60537e00fea4e8c6b535b9359498c9c0ee9d912fedca52562c6f0bdae78c4a"

check_file_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/catalog/playbooks.go" \
  "openapi/paths/catalog/playbooks.go" \
  "f6a66c1873062dde7c6d7a3ef2bbc10e8293665f9e3971a61a3274f1909d2d46"

# D4 subsets: the evidence routes entry drops core's dangling 501
# UnsupportedCapability response (no definition at the pin — porting it
# would dangle the served document); the deployable-unit entry ports
# alone from the shared investigations file; the repository subset drops
# the mux-health entry (dest serves its own probe shape) and the
# schema-interpolated story entry (no line-verbatim port without a core
# import). Every remaining payload line is verbatim in the pinned file.
check_fragment_subset_named_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/evidence/routes.go" \
  "openapi/paths/evidence/evidence_routes.go" \
  "8c9950983d889af526fcd886ee02944f9179c99fde60c191c972c53605df563a" \
  "EvidenceRoutes"

check_fragment_subset_named_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/evidence/investigations.go" \
  "openapi/paths/evidence/deployable_unit.go" \
  "df0e51563c025a4c6571f31a04704c9abce4b9e33e613baadac5d4e316924cb0" \
  "DeployableUnitPacket"

check_fragment_subset_named_at "$SRC_SHA_WAVE" "go/internal/query/openapi/paths/repository/routes.go" \
  "openapi/paths/repository/repository_routes.go" \
  "3641f90522c313023386476d92fce558219284e0cc76c886b6b6ecd07527ae65" \
  "RepositoryRoutes"

# The doc-UI shells ride their const openers (`const swaggerUIHTML =
# `<!doctype html>` in core): no standalone doctype line exists to
# line-match, so this gate compares the whole shell block instead. The
# dest const name is dest-owned; the block bytes after the opener must
# equal the pinned core block bytes exactly.
check_docui_shell_at() {
  local sha="$1" core_const="$2" dest_rel="$3" dest_const="$4"
  local fetched
  fetched="$(fetch_at "$sha" "go/internal/query/openapi/spec.go")"
  local got_hash
  got_hash="$(sha256sum "$fetched" | cut -d' ' -f1)"
  if [[ "$got_hash" != "6d721a03c27feb55720381ae3db8b40f8f4cc53f267a3830ff7cbcaff201ad80" ]]; then
    echo "FAIL: go/internal/query/openapi/spec.go at ${sha} hash ${got_hash} != recorded 6d721a03c27feb55720381ae3db8b40f8f4cc53f267a3830ff7cbcaff201ad80" >&2
    exit 1
  fi
  block_of() { # file, const opener: payload lines after the opener to the closing backtick
    awk -v c="$1" 'index($0, c)==1{flag=1; sub(/^[^\x60]*\x60/, ""); print; next} flag{print} flag && /^\x60$/ {exit}' "$2"
  }
  if ! cmp -s <(block_of "const ${core_const} = " "$fetched") <(block_of "const ${dest_const} = " "${REPO_ROOT}/${dest_rel}"); then
    echo "FAIL: ${dest_rel} ${dest_const} block differs from ${core_const} at ${sha}" >&2
    diff <(block_of "const ${core_const} = " "$fetched") <(block_of "const ${dest_const} = " "${REPO_ROOT}/${dest_rel}") >&2 || true
    exit 1
  fi
  echo "OK: ${dest_rel} ${dest_const} block identical to ${core_const} at ${sha}"
}

check_docui_shell_at "$SRC_SHA_WAVE" "swaggerUIHTML" \
  "api/leafb_terminate.go" "leafbSwaggerUIShell"

check_docui_shell_at "$SRC_SHA_WAVE" "redocHTML" \
  "api/leafb_terminate.go" "leafbReDocShell"

# Wave components (Decision 5(c)): every ported line of the D2 block in
# openapi/components.json — the parameters/ServiceName entry, the
# AdminProviderConfig and AdminProviderConfigWriteRequest schemas, and
# the Unauthorized and NotImplemented responses — must appear verbatim
# in its pinned component source. The served-spec LocalRefsResolve
# test independently requires the entries.
check_leafb_components_at() {
  local fail=0
  check_block() { # dest entry marker, src sha, src path
    local marker="$1" sha="$2" path="$3"
    local fetched
    fetched="$(fetch_at "$sha" "$path")"
    local block
    block="$(awk -v m="$marker" '$0 ~ m{flag=1} flag{print} flag && /^      \},?$/ && ++closes==1{exit}' "${REPO_ROOT}/openapi/components.json")"
    if [[ -z "$block" ]]; then
      echo "FAIL: no $marker block in openapi/components.json" >&2
      return 1
    fi
    # The block's first line rides the Go const opener in core (no
    # standalone line exists there), so it matches as a substring;
    # every following line must match whole-line. The served-spec
    # validity and LocalRefsResolve tests independently pin the entry
    # name and shape.
    local first=1
    while IFS= read -r line; do
      [[ -z "$line" ]] && continue
      if [[ "$first" == "1" ]]; then
        first=0
        if ! grep -qF -- "$line" "$fetched"; then
          echo "FAIL: components opener missing from ${path} at ${sha}: ${line}" >&2
          return 1
        fi
        continue
      fi
      if ! grep -qFx -- "$line" "$fetched"; then
        echo "FAIL: components line missing from ${path} at ${sha}: ${line}" >&2
        return 1
      fi
    done <<< "$block"
  }
  check_block '"ServiceName"' "$SRC_SHA_WAVE" "go/internal/query/openapi/components.go" \
    || fail=1
  check_block '"AdminProviderConfig"' "$SRC_SHA_PROVIDERCONFIG" "go/internal/query/openapi/components_provider_configs.go" \
    || fail=1
  check_block '"AdminProviderConfigWriteRequest"' "$SRC_SHA_PROVIDERCONFIG" "go/internal/query/openapi/components_provider_configs.go" \
    || fail=1
  check_block '"Unauthorized"' "$SRC_SHA_WAVE" "go/internal/query/openapi/components_responses.go" \
    || fail=1
  check_block '"NotImplemented"' "$SRC_SHA_WAVE" "go/internal/query/openapi/components_responses.go" \
    || fail=1
  if [[ "$fail" != "0" ]]; then
    exit 1
  fi
  echo "OK: wave components lines all verbatim in pinned component sources"
}

check_leafb_components_at

# D4 wave components (Decision 5(c)): every ported line of the D4 block
# in openapi/components.json — the RepoId parameter, the
# CollectorReadinessEnvelope, Repository, RepositoryRef, CatalogWorkload,
# InvestigationEvidencePacket, Relationship, and ActiveWorkSource
# schemas, and the GatewayTimeout response — must appear verbatim in
# its pinned component source (same block rule as the D2 gate: the
# first line matches as a substring, every following line whole-line).
check_d4_components_at() {
  local fail=0
  check_block() { # dest entry marker, src sha, src path
    local marker="$1" sha="$2" path="$3"
    local fetched
    fetched="$(fetch_at "$sha" "$path")"
    local block
    block="$(awk -v m="$marker" '$0 ~ m{flag=1} flag{print} flag && /^      \},?$/ && ++closes==1{exit}' "${REPO_ROOT}/openapi/components.json")"
    if [[ -z "$block" ]]; then
      echo "FAIL: no $marker block in openapi/components.json" >&2
      return 1
    fi
    local first=1
    while IFS= read -r line; do
      [[ -z "$line" ]] && continue
      if [[ "$first" == "1" ]]; then
        first=0
        if ! grep -qF -- "$line" "$fetched"; then
          echo "FAIL: components opener missing from ${path} at ${sha}: ${line}" >&2
          return 1
        fi
        continue
      fi
      if ! grep -qFx -- "$line" "$fetched"; then
        echo "FAIL: components line missing from ${path} at ${sha}: ${line}" >&2
        return 1
      fi
    done <<< "$block"
  }
  check_block '"RepoId": {' "$SRC_SHA_WAVE" "go/internal/query/openapi/components.go" \
    || fail=1
  check_block '"CollectorReadinessEnvelope": {' "$SRC_SHA_WAVE" "go/internal/query/openapi/components.go" \
    || fail=1
  check_block '"Repository": {' "$SRC_SHA_WAVE" "go/internal/query/openapi/components.go" \
    || fail=1
  check_block '"RepositoryRef": {' "$SRC_SHA_WAVE" "go/internal/query/openapi/components.go" \
    || fail=1
  check_block '"CatalogWorkload": {' "$SRC_SHA_WAVE" "go/internal/query/openapi/components.go" \
    || fail=1
  check_block '"InvestigationEvidencePacket": {' "$SRC_SHA_WAVE" "go/internal/query/openapi/components.go" \
    || fail=1
  check_block '"Relationship": {' "$SRC_SHA_WAVE" "go/internal/query/openapi/components.go" \
    || fail=1
  check_block '"ActiveWorkSource": {' "$SRC_SHA_WAVE" "go/internal/query/openapi/components_active_work_source.go" \
    || fail=1
  check_block '"GatewayTimeout": {' "$SRC_SHA_WAVE" "go/internal/query/openapi/components_responses.go" \
    || fail=1
  if [[ "$fail" != "0" ]]; then
    exit 1
  fi
  echo "OK: D4 components lines all verbatim in pinned component sources"
}

check_d4_components_at

# D6 Status + SupplyChain reads wave (Decision 5(c)): the wave
# fragments are byte-identical ports (whole-file) or line-verbatim
# subsets of the pinned core files at SRC_SHA_D6. The two finding
# fragments ride the dest-owned exported wrappers (ImpactFindings,
# ImpactExplain) over the core unexported consts, sharing the
# byte-identical runtime_context.go sibling; the status reads fragment
# ports only the seven lane entries, never a relayed key.
check_file_at "$SRC_SHA_D6" "go/internal/query/openapi/paths/status/collector_readiness.go" \
  "openapi/paths/status/collector_readiness.go" \
  "12f2e23ec324531f83c960eb9dc429c1da11f749cdce000e14b659d476da6f2f"

check_file_at "$SRC_SHA_D6" "go/internal/query/openapi/paths/status/operator_control_plane.go" \
  "openapi/paths/status/operator_control_plane.go" \
  "fc58c514a42d02b56bd879f1b74678c3ee3c494bf8be49ef129525abda0f28c9"

check_file_at "$SRC_SHA_D6" "go/internal/query/openapi/paths/status/semantic.go" \
  "openapi/paths/status/semantic.go" \
  "c50a5391e3d765cdf3d40f9999eae33ffa1e1a299df455f81903af63272578da"

check_file_at "$SRC_SHA_D6" "go/internal/query/openapi/paths/freshness/causality.go" \
  "openapi/paths/freshness/causality.go" \
  "6409c05ee4b9c5078a7acabb12c3b783a297e2505a316f9b71d3bb2db5b0d3c2"

check_file_at "$SRC_SHA_D6" "go/internal/query/openapi/paths/supply/chain/runtime_context.go" \
  "openapi/paths/supply/chain/runtime_context.go" \
  "ec83120edab5d9b4d75622cc748d24c8f88f06036f8bc23e377a8b9830e6b83b"

check_file_at "$SRC_SHA_D6" "go/internal/query/openapi/paths/supply/chain/advisory_catalog.go" \
  "openapi/paths/supply/chain/advisory_catalog.go" \
  "2316253e4c20862081a21778f2c6619bb4e1e35ba63de7c603a8b4069e122d3b"

check_file_at "$SRC_SHA_D6" "go/internal/query/openapi/paths/supply/chain/advisory_evidence.go" \
  "openapi/paths/supply/chain/advisory_evidence.go" \
  "4a69d35575e23d8e8f7ed4453dbbd14c453688579e1f8edb320e35d74609d212"

check_file_at "$SRC_SHA_D6" "go/internal/query/openapi/paths/supply/chain/container_images.go" \
  "openapi/paths/supply/chain/container_images.go" \
  "29ca221b684f1f3f1b88042121090f5f7ce136dae6812f8071e5dc911d1c42a1"

check_file_at "$SRC_SHA_D6" "go/internal/query/openapi/paths/supply/chain/container_image_identity_aggregate.go" \
  "openapi/paths/supply/chain/container_image_identity_aggregate.go" \
  "457eaadc8b6c325b96bb399610577c2145fd7d77ba4e9902b8286f71b7b07600"

check_file_at "$SRC_SHA_D6" "go/internal/query/openapi/paths/supply/chain/impact_aggregate.go" \
  "openapi/paths/supply/chain/impact_aggregate.go" \
  "9bcd0f907ab0012d86a79ee1a82339c16fd39a2535dec58d138cf625e61fec21"

check_file_at "$SRC_SHA_D6" "go/internal/query/openapi/paths/supply/chain/sbom_attestations.go" \
  "openapi/paths/supply/chain/sbom_attestations.go" \
  "9571183b1fd617e39ecf214e5b9a679dcb73342b0b7f8c68c7a380f151fa710b"

check_file_at "$SRC_SHA_D6" "go/internal/query/openapi/paths/supply/chain/sbom_attestation_attachment_aggregate.go" \
  "openapi/paths/supply/chain/sbom_attestation_attachment_aggregate.go" \
  "c8aad4c90a5a1f0973b542c172e9e492582ef82ae2112614a7170f34962cf856"

check_file_at "$SRC_SHA_D6" "go/internal/query/openapi/paths/supply/chain/security_alerts.go" \
  "openapi/paths/supply/chain/security_alerts.go" \
  "3f8431bd296d43ab284ee97f3ba084bafcf2ae55f12d06ff3b7bf0473840b491"

check_file_at "$SRC_SHA_D6" "go/internal/query/openapi/paths/supply/chain/security_alert_aggregate.go" \
  "openapi/paths/supply/chain/security_alert_aggregate.go" \
  "8a5709e57dd25f9263274828c4a470baa24236abf00a5de331a518a46dfd5088"

check_file_at "$SRC_SHA_D6" "go/internal/query/openapi/paths/supply/chain/vulnerability_scanner_contract.go" \
  "openapi/paths/supply/chain/vulnerability_scanner_contract.go" \
  "3dbfbc45e39e80ba183ee7f0a45acd40cf1cd0de596fdd99dff63420326a6ca8"

check_fragment_subset_named_at "$SRC_SHA_D6" "go/internal/query/openapi/paths/status/routes.go" \
  "openapi/paths/status/status_reads.go" \
  "24a57b35c675cb33ad5011343a0aeb68d49e9d05d7645f397de2d88e8e5afa61" \
  "StatusReads"

check_fragment_subset_named_at "$SRC_SHA_D6" "go/internal/query/openapi/paths/supply/chain/impact_explain.go" \
  "openapi/paths/supply/chain/impact_explain.go" \
  "7b64a8ce69cc2802157303f1fd50a2914a5cd2c9693743368da0889639852f3e" \
  "ImpactExplain"

check_fragment_subset_named_at "$SRC_SHA_D6" "go/internal/query/openapi/paths/supply/chain/impact_findings.go" \
  "openapi/paths/supply/chain/impact_findings.go" \
  "0b0adcf656d2d14b8c470c3e46511d3e8d800a74dde92e68c4ffdde7278dce67" \
  "ImpactFindings"

check_d6_components_at() {
  local fail=0
  check_block() { # dest entry marker, src sha, src path
    local marker="$1" sha="$2" path="$3"
    local fetched
    fetched="$(fetch_at "$sha" "$path")"
    local block
    block="$(awk -v m="$marker" '$0 ~ m{flag=1} flag{print} flag && /^      \},?$/ && ++closes==1{exit}' "${REPO_ROOT}/openapi/components.json")"
    if [[ -z "$block" ]]; then
      echo "FAIL: no $marker block in openapi/components.json" >&2
      return 1
    fi
    local first=1
    while IFS= read -r line; do
      [[ -z "$line" ]] && continue
      if [[ "$first" == "1" ]]; then
        first=0
        if ! grep -qF -- "$line" "$fetched"; then
          echo "FAIL: components opener missing from ${path} at ${sha}: ${line}" >&2
          return 1
        fi
        continue
      fi
      if ! grep -qFx -- "$line" "$fetched"; then
        echo "FAIL: components line missing from ${path} at ${sha}: ${line}" >&2
        return 1
      fi
    done <<< "$block"
  }
  check_block '"CoordinatorSource": {' "$SRC_SHA_D6" "go/internal/query/openapi/components_active_work_source.go" \
    || fail=1
  check_block '"TerraformStateSource": {' "$SRC_SHA_D6" "go/internal/query/openapi/components_active_work_source.go" \
    || fail=1
  if [[ "$fail" != "0" ]]; then
    exit 1
  fi
  echo "OK: D6 components lines all verbatim in pinned component sources"
}

check_d6_components_at

check_d8_components_at() {
  local fail=0
  check_block() { # dest entry marker, src sha, src path
    local marker="$1" sha="$2" path="$3"
    local fetched
    fetched="$(fetch_at "$sha" "$path")"
    local block
    block="$(awk -v m="$marker" '$0 ~ m{flag=1} flag{print} flag && /^      \},?$/ && ++closes==1{exit}' "${REPO_ROOT}/openapi/components.json")"
    if [[ -z "$block" ]]; then
      echo "FAIL: no $marker block in openapi/components.json" >&2
      return 1
    fi
    local first=1
    while IFS= read -r line; do
      [[ -z "$line" ]] && continue
      if [[ "$first" == "1" ]]; then
        first=0
        if ! grep -qF -- "$line" "$fetched"; then
          echo "FAIL: components opener missing from ${path} at ${sha}: ${line}" >&2
          return 1
        fi
        continue
      fi
      if ! grep -qFx -- "$line" "$fetched"; then
        echo "FAIL: components line missing from ${path} at ${sha}: ${line}" >&2
        return 1
      fi
    done <<< "$block"
  }
  check_block '"LocalIdentityBootstrapRequest": {' "$SRC_SHA_D8" "go/internal/query/openapi/components_local_identity.go" \
    || fail=1
  check_block '"LocalIdentityLoginRequest": {' "$SRC_SHA_D8" "go/internal/query/openapi/components_local_identity.go" \
    || fail=1
  check_block '"LocalIdentityInvitationRequest": {' "$SRC_SHA_D8" "go/internal/query/openapi/components_local_identity.go" \
    || fail=1
  check_block '"LocalIdentityInvitationAcceptanceRequest": {' "$SRC_SHA_D8" "go/internal/query/openapi/components_local_identity.go" \
    || fail=1
  check_block '"LocalIdentityPasswordRotationRequest": {' "$SRC_SHA_D8" "go/internal/query/openapi/components_local_identity.go" \
    || fail=1
  check_block '"LocalIdentityBreakGlassRequest": {' "$SRC_SHA_D8" "go/internal/query/openapi/components_local_identity.go" \
    || fail=1
  check_block '"LocalIdentityAPITokenCreateRequest": {' "$SRC_SHA_D8" "go/internal/query/openapi/components_local_identity.go" \
    || fail=1
  check_block '"LocalIdentityAPITokenResponse": {' "$SRC_SHA_D8" "go/internal/query/openapi/components_local_identity.go" \
    || fail=1
  check_block '"LocalIdentitySessionResponse": {' "$SRC_SHA_D8" "go/internal/query/openapi/components_local_identity.go" \
    || fail=1
  check_block '"BrowserSessionAuth": {' "$SRC_SHA_D8" "go/internal/query/openapi/components_workload_session.go" \
    || fail=1
  check_block '"BrowserSessionResponse": {' "$SRC_SHA_D8" "go/internal/query/openapi/components_workload_session.go" \
    || fail=1
  if [[ "$fail" != "0" ]]; then
    exit 1
  fi
  echo "OK: D8 components lines all verbatim in pinned component sources"
}

check_d8_components_at

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
  "go/internal/query/sign_in_policy_types.go" \
  "go/internal/query/sign_in_policy_mutations.go"

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
  "go/internal/query/admin/identity/types.go" \
  "go/internal/query/admin/identity/mutations.go"

check_excerpt_at "$SRC_SHA_SETUP" "contracts/upstream/setup-wire-shapes.txt" \
  "go/internal/query/setup_types.go"

check_excerpt_at "$SRC_SHA_PROVIDERCONFIG" "contracts/upstream/provider-config-wire-shapes.txt" \
  "go/internal/query/admin/provider/config/admission.go" \
  "go/internal/query/admin/provider/config/types.go" \
  "go/internal/query/admin/provider/config/mutations.go" \
  "go/internal/query/admin/provider/config/audit.go"

check_excerpt_at "$SRC_SHA_AUTHLOGIN" "contracts/upstream/oidc-login-wire-shapes.txt" \
  "go/internal/query/oidc_login_handler.go" \
  "go/cmd/api/oidc_login.go"

check_excerpt_at "$SRC_SHA_AUTHLOGIN" "contracts/upstream/github-login-wire-shapes.txt" \
  "go/internal/query/github_login_handler.go" \
  "go/cmd/api/github_login.go"

check_excerpt_at "$SRC_SHA_SAMLLOGIN" "contracts/upstream/saml-login-wire-shapes.txt" \
  "go/internal/query/saml_admission.go" \
  "go/internal/query/saml_handler.go"

check_excerpt_at "$SRC_SHA_RECOVERY" "contracts/upstream/recovery-wire-shapes.txt" \
  "go/internal/query/admin/recovery_admission.go" \
  "go/internal/query/admin/handler.go"

check_excerpt_at "$SRC_SHA_D1" "contracts/upstream/admin-read-wire-shapes.txt" \
  "go/internal/query/admin/read_admission.go" \
  "go/internal/query/admin/handler.go"

check_excerpt_at "$SRC_SHA_D1" "contracts/upstream/admin-identity-read-wire-shapes.txt" \
  "go/internal/query/admin/identity/read_admission.go" \
  "go/internal/query/admin/identity/governance_audit_read_admission.go" \
  "go/internal/query/admin/identity/types.go"

check_excerpt_at "$SRC_SHA_D1" "contracts/upstream/terminal-failure-read-wire-shapes.txt" \
  "go/internal/query/admin/acknowledgement/terminal_failure_read_admission.go"

check_excerpt_at "$SRC_SHA_D6" "contracts/upstream/status-read-wire-shapes.txt" \
  "go/internal/status/read_admission.go" \
  "go/internal/status/report.go"

check_excerpt_at "$SRC_SHA_D6" "contracts/upstream/supply-chain-read-wire-shapes.txt" \
  "go/internal/query/supply/chain/supply_read_admission.go"

# Version pins referenced by the contract records.
grep -qF 'const CoreReadContractsVersion = "v0"' "${REPO_ROOT}/contracts/upstream/api_boundary.go"
grep -qF 'SuppressionAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/suppression-wire-shapes.txt"
grep -qF 'TerminalFailureAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/terminal-failure-wire-shapes.txt"
grep -qF 'ReplayLedgerAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/replay-ledger-wire-shapes.txt"
grep -qF 'SignInPolicyAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/sign-in-wire-shapes.txt"
grep -qF 'RetirementAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/retirement-wire-shapes.txt"
grep -qF 'AdminIdentityAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/admin-identity-wire-shapes.txt"
grep -qF 'SetupAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/setup-wire-shapes.txt"
grep -qF 'ProviderConfigMutationAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/provider-config-wire-shapes.txt"
grep -qF 'ProviderConfigReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/provider-config-wire-shapes.txt"
grep -qF 'OIDCLoginAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/oidc-login-wire-shapes.txt"
grep -qF 'GitHubLoginAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/github-login-wire-shapes.txt"
grep -qF 'SAMLLoginAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/saml-login-wire-shapes.txt"
grep -qF 'RecoveryAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/recovery-wire-shapes.txt"
grep -qF 'AdminReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/admin-read-wire-shapes.txt"
grep -qF 'AdminIdentityReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/admin-identity-read-wire-shapes.txt"
grep -qF 'GovernanceAuditReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/admin-identity-read-wire-shapes.txt"
grep -qF 'TerminalFailureReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/terminal-failure-read-wire-shapes.txt"
grep -qF 'StatusReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/status-read-wire-shapes.txt"
grep -qF 'SupplyChainReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/supply-chain-read-wire-shapes.txt"
grep -qF 'SessionResolverAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/session_admission.go"
grep -qF 'BrowserSessionAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/session_admission.go"
grep -qF 'LocalIdentityAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/session_admission.go"
grep -qF 'APITokenAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/session_admission.go"
echo "OK: version pins v0 present"

# No-core-import proof (native-dispatch pilot, arbiter ruling (a)): the
# runtime module (api, internal, cmd) must not import eshu-hq/eshu core
# packages. Dest dispatches over HTTP write-through; the vendored
# contracts/upstream copies are parity evidence, not build inputs (their
# own go.mod is a separate module). Fails closed on any core import AND
# on a listing-tool failure: a suppressed stderr that yields an empty
# list must never read as proof.
if ! go list -deps ./api/... ./internal/... ./cmd/... >"${tmpdir}/deps" 2>"${tmpdir}/deps.err"; then
  echo "FAIL: go list -deps failed, no-core-import unproven" >&2
  cat "${tmpdir}/deps.err" >&2
  exit 1
fi
if grep -q '^github\.com/eshu-hq/eshu\(/\|$\)' "${tmpdir}/deps"; then
  echo "FAIL: runtime packages import eshu-hq/eshu core code (HTTP write-through only)" >&2
  grep '^github\.com/eshu-hq/eshu\(/\|$\)' "${tmpdir}/deps" >&2
  exit 1
fi
echo "OK: runtime packages import no eshu-hq/eshu core code"

echo "parity: all checks passed"
