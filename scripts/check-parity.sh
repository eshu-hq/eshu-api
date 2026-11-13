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
SRC_SHA_SETUP="ba038ecaf20352f7dacac2fc6b5ace66497cfff1"
SRC_SHA_PROVIDERCONFIG="99381691b1ac49254e813e5a013cfb654d432491"
SRC_SHA_AUTHLOGIN="1e03478614fc54882b9616d37ea9103ea5ada6f4"
SRC_SHA_SAMLLOGIN="aac271ad78e328462dabc7518d5f7e5739bdc651"
SRC_SHA_RECOVERY="7c60583996d9a3ccc46288f40a38f614a4f28ea8"
SRC_SHA_D1="2c89a9d8b0a5efa1c4fbdd1d3e24de00012d1923"
SRC_SHA_D0="ec1643d67f"
SRC_SHA_WAVE="17feac86e089a6da208fe6c93b5d89b8fb7ef3cc"
SRC_SHA_D6="6b66e6b23efe226970b4571c955aed1dc42f987a"
SRC_SHA_STATUS_EXTRA="0ab602df5cda205dc326a44fae02c383c5a653b3"
SRC_SHA_D7="19de7346d11a184bf88d8f307b5d7a9ced0a4657"
SRC_SHA_D8="aa58debdd8102042f6c7e8b8e2e2944805889ef3"
SRC_SHA_D9="40e88be128ca876f7333588137af5a780bf31b93"
SRC_SHA_D9_GITHUB="8c2324693320c8f69ef3e8078c795956b549d7ad"
SRC_SHA_D9_SAML="8c2324693320c8f69ef3e8078c795956b549d7ad"
SRC_SHA_GAP02="4a587431fdf748c0a231b60708131fbaa1f41064"
SRC_SHA_COMPARE="3870181f323dbac84496efb87f7498b19afe1bf4"
SRC_SHA_O2O6="14a5bf8a52c582360158f85abda4c9675b2ea2e6"
SRC_SHA_WAVE1="51b84b23a4d21fb1282c3a50f5f1530ad5853c51"
SRC_SHA_WAVE2_CONTENT="c987f7357be6e779676a4d922f456e788b068a3f"
SRC_SHA_W6A="e504e328a7b665d7a5b4358a2f8b2ce5201b1d72"
SRC_SHA_W8A="e504e328a7b665d7a5b4358a2f8b2ce5201b1d72"
SRC_SHA_W4="e504e328a7b665d7a5b4358a2f8b2ce5201b1d72"
SRC_SHA_DRIFT="d81bd1280212745a2a3215f988087e1b92bd828"
SRC_SHA_RETIREUNCLAIMED="bc150cc0c6ff38cc0752f8a588af2afb5fafa9ae"
SRC_SHA_W3SUPP="d9deb65433e8c3c2ed29a242aeaccec83a9c33e3"
SRC_SHA_DEAD="8b28c0dba5f12c3b1b11c90fac6b24cfe5d82875"
SRC_SHA_GRAPH="0fd6db6deb38beaa9327ba015af5bd279729f8ae"
SRC_SHA_DIVERGENCE="d584e3897ed029611a0c2224edae282a591e0b34"

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

# Retire-unclaimed writer pin (eshu-hq/eshu#10583, unwired seam): the
# frozen method-set test, byte-identical to the merge commit.
check_file_at "$SRC_SHA_RETIREUNCLAIMED" "go/internal/query/admin/retire_unclaimed_admission_contract_test.go" \
  "contracts/upstream/retire_unclaimed_admission_contract_test.go" \
  "22b861314bdecf674047c8e642e59e58f64d6517a653f6e9288ae088066a86d2"

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

# D6 phase-2 extra-port Status pin (eshu-hq/eshu#10405, unwired seam):
# the frozen method-set test, byte-identical to the merge commit.
check_file_at "$SRC_SHA_STATUS_EXTRA" "go/internal/query/status_extra_read_admission_contract_test.go" \
  "contracts/upstream/status_extra_read_admission_contract_test.go" \
  "12615d35aebc9ef4e436a0ee616915ed584ab848602ddf62adf554acd15368d7"

# Wave-1 Freshness read-admission pin (eshu-hq/eshu#10494, unwired
# seam): the frozen method-set test, byte-identical to the merge
# commit. The pin re-declares (never embeds) exactly the three
# StatusStore signatures the freshness drilldown routes admit through.
check_file_at "$SRC_SHA_WAVE1" "go/internal/query/freshness/read_admission_contract_test.go" \
  "contracts/upstream/freshness_read_admission_contract_test.go" \
  "af566dadb76cd0a4f2caff92434ad3f62956b8b2f62520e5573e7baf48b92b80"

# Wave-1 Freshness spec (Decision 5(c)): the three drilldown OpenAPI
# fragments are whole-file byte-identical ports of the pinned core
# files at the Wave-1 merge. All three are standalone entries — not
# paths-closing entries shaped like the Compare fragment — so no
# disclosed absence was needed.
check_file_at "$SRC_SHA_WAVE1" "go/internal/query/openapi/paths/freshness/changed_since.go" \
  "openapi/paths/freshness/changed_since.go" \
  "26db6d16d8936ab0398ccd94e447305eedbe98fb0707a87e9f87e8e1d11fb9e0"

check_file_at "$SRC_SHA_WAVE1" "go/internal/query/openapi/paths/freshness/generations.go" \
  "openapi/paths/freshness/generations.go" \
  "acb3e8f332b3b0524a2b0cc52fe559dce8827471bfa87b651050e16df3303845"

check_file_at "$SRC_SHA_WAVE1" "go/internal/query/openapi/paths/freshness/service_changed_since.go" \
  "openapi/paths/freshness/service_changed_since.go" \
  "e15d5b3dfeb864d0384893c58752a9d40bb49e8e63a24af0308b12b6d2243a05"

# Wave-2 Content point-read pin (eshu-hq/eshu#10504, unwired seam):
# the frozen method-set test, byte-identical to the merge commit. The
# pin re-declares (never embeds) exactly the three ContentStore
# signatures the content point routes admit through.
check_file_at "$SRC_SHA_W8A" "go/internal/query/secrets/read_admission_contract_test.go" \
  "contracts/upstream/secretsiam_read_admission_contract_test.go" \
  "c761d01fa4ce4812689376357f54bb1532b4dcfb5511a3145f20724a7ec612ed"

check_file_at "$SRC_SHA_W8A" "go/internal/query/status_governance_read_admission_contract_test.go" \
  "contracts/upstream/status_governance_read_admission_contract_test.go" \
  "a6d85284bebd40dd68d9ee890903725fcaf768a51e599714446b2057a661f8b1"

check_file_at "$SRC_SHA_W8A" "go/internal/query/status_hosted_readiness_read_admission_contract_test.go" \
  "contracts/upstream/status_hosted_readiness_read_admission_contract_test.go" \
  "939379f0b4059cd6d95db39e5948273ec1d00c0e9ad726fb617874526eaf8f79"

check_file_at "$SRC_SHA_WAVE2_CONTENT" "go/internal/query/content_read_admission_contract_test.go" \
  "contracts/upstream/content_point_read_admission_contract_test.go" \
  "c46ef2c13d23b1e654b473b61e29a24a82cc35782cc0e948bd6c62cba94434a6"

# Wave-2 Content spec (Decision 5(c)): the three point-read entries
# are a line-verbatim subset of the pinned core content fragment at
# the Wave-2 merge. Core ships the lane entries mid-list with the
# search entries in one file, so no whole-file identity port can serve
# this lane only; the gate pins the core file's sha256 and proves
# every payload line of the dest fragment (the const string body,
# which is the only JSON the assembly consumes) appears verbatim in
# the pinned core file. The search entries stay core-side with the
# still-relaying search legs, so no served key duplicates a relayed
# route.
check_fragment_subset_named_at "$SRC_SHA_WAVE2_CONTENT" "go/internal/query/openapi/paths/search/content.go" \
  "openapi/paths/search/content_point_reads.go" \
  "d8254d1b9313db556f69b3d9b40925fc4a64094aa6629aa47dad16aa728edcca" \
  "ContentPointReads"

# W4 Content search spec (satellite eshu-hq/eshu-api#111): the four
# search entries are the same line-verbatim mid-list subset pattern —
# the core file is byte-identical between the Wave-2 SHA and the W4
# base, so the recorded whole-file hash repeats. The dest fragment is a
# NEW wave file, never an extension of the point-reads file.
check_fragment_subset_named_at "$SRC_SHA_W4" "go/internal/query/openapi/paths/search/content.go" \
  "openapi/paths/search/content_search.go" \
  "d8254d1b9313db556f69b3d9b40925fc4a64094aa6629aa47dad16aa728edcca" \
  "ContentSearch"

# W6a Code-flow pin (eshu-hq/eshu#10552, unwired seam): the frozen
# method-set test, byte-identical to the merge commit at e504e328a7.
# The pin re-declares (never embeds) exactly the one CodeFlowStore
# signature the four Code-flow routes admit through.
check_file_at "$SRC_SHA_W6A" "go/internal/query/codequery/code_flow_read_admission_contract_test.go" \
  "contracts/upstream/code_flow_read_admission_contract_test.go" \
  "2c7b1e5e9e5527ba91cb6ce578eeb2ed098b9408876e612bfc7b58f71efeaa79"

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

# Native spec (Decision 5(c)): the retire-unclaimed OpenAPI fragment is
# a byte-identical port of the pinned core file. The fragment predates
# the RetireUnclaimedAdmission v0 pin (minted via eshu-hq/eshu#10553)
# and is byte-identical at the pin merge, so the gate pins it there.
check_file_at "$SRC_SHA_RETIREUNCLAIMED" "go/internal/query/openapi/paths/status/workflow_retire_unclaimed.go" \
  "openapi/paths/status/workflow_retire_unclaimed.go" \
  "b58ed9e3ee80e774b022af5d42cd649fb9427697ed202bb1ea2a8288bb940f30"

# D8 session-resolver pins (satellite D8 wave, arbiter
# APPROVED-WITH-CONDITIONS): the four v0 admission pins
# (SessionResolver, BrowserSession, LocalIdentity, APIToken), unwired
# seams, recorded byte-identical satellite-side first (D7 #57
# precedent), re-anchored at the fresh origin/main SHA (byte-identical
# to the stale 91f2df28db pin, same sha256 — re-anchor, no content
# change; every other file gated at SRC_SHA_D8 verifies identical at
# both SHAs).
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

# D9 OIDC-login wave (satellite D9 OIDC lane, arbiter
# APPROVED-WITH-CONDITIONS): the v0 admission pin re-anchored at the
# fresh wave SHA (byte-identical to the stale SRC_SHA_AUTHLOGIN pin,
# same sha256 — re-anchor, no content change), the wire-shapes excerpt
# re-anchored at the same SHA, and the OIDC login + callback entries
# as a line-verbatim subset of the pinned core routes.go (byte-identical
# to the D8 pin file, same sha256). The GitHub/SAML pins stay at their
# stale anchors until their own sequential lanes.
check_file_at "$SRC_SHA_D9" "go/internal/query/oidc_login_admission_contract_test.go" \
  "contracts/upstream/oidc_login_admission_contract_test.go" \
  "03342f7ad0d28aef72583ce21a7747fe360fe755367fef40660e58361f19d2c0"

# NOTE: the D9 wire-shapes excerpt gate lives with the other
# check_excerpt_at calls below (that function is defined after this
# block; calling it here fails with command-not-found).

check_fragment_subset_named_at "$SRC_SHA_D9" "go/internal/query/openapi/paths/auth/routes.go" \
  "openapi/paths/auth/oidc_login.go" \
  "709fe6b3e96967fa615bff59a8c02cb8c1c5fbc6a05744bf57835170a5ad633a" \
  "OIDCLogin"

# D9 GitHub-login wave (satellite D9 GitHub lane, arbiter
# APPROVED-WITH-CONDITIONS): the v0 admission pin re-anchored at the
# fresh wave SHA (byte-identical to the stale SRC_SHA_AUTHLOGIN pin,
# same sha256 — re-anchor, no content change), the wire-shapes excerpt
# re-anchored at the same SHA, and the GitHub login + callback entries
# as a line-verbatim subset of the pinned core routes.go (byte-identical
# to the OIDC wave file, same sha256). The SAML pin stays at its stale
# anchor until its own sequential lane.
check_file_at "$SRC_SHA_D9_GITHUB" "go/internal/query/github_login_admission_contract_test.go" \
  "contracts/upstream/github_login_admission_contract_test.go" \
  "f6a7fc9920176c76c8cf6e5ec223e881323564405b5931770d43db9f7256889c"

check_fragment_subset_named_at "$SRC_SHA_D9_GITHUB" "go/internal/query/openapi/paths/auth/routes.go" \
  "openapi/paths/auth/github_login.go" \
  "709fe6b3e96967fa615bff59a8c02cb8c1c5fbc6a05744bf57835170a5ad633a" \
  "GitHubLogin"

# D9 SAML-login wave (satellite D9 SAML lane, arbiter
# APPROVED-WITH-CONDITIONS): the v0 admission pin re-anchored at the
# fresh wave SHA (byte-identical to the stale SRC_SHA_SAMLLOGIN pin,
# same sha256 — re-anchor, no content change; the wave SHA is unchanged
# from the GitHub lane because `git diff aac271ad..8c23246933` over the
# 5 SAML files is empty), the wire-shapes excerpt re-anchored at the
# same SHA, and the SAML metadata + login + ACS entries as a
# line-verbatim subset of the pinned core routes.go (byte-identical to
# the OIDC/GitHub wave file, same sha256). The D9 wave is complete with
# this lane.
check_file_at "$SRC_SHA_D9_SAML" "go/internal/query/saml_login_admission_contract_test.go" \
  "contracts/upstream/saml_login_admission_contract_test.go" \
  "bc04787bd208bf07428f4a9bc4660946342c7e4f3b60121202cb7f0a070556d0"

check_fragment_subset_named_at "$SRC_SHA_D9_SAML" "go/internal/query/openapi/paths/auth/routes.go" \
  "openapi/paths/auth/saml_login.go" \
  "709fe6b3e96967fa615bff59a8c02cb8c1c5fbc6a05744bf57835170a5ad633a" \
  "SAMLLogin"

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

# D3 Gap 02 trio (satellite eshu-hq/eshu-api#70, core eshu-hq/eshu#10412
# merge 4a587431fd): the three versioned admission pins (unwired-seam
# style — version constant plus frozen method-set/shape contract test
# per pin), the two whole-file OpenAPI ports, and the tuning-report
# line-verbatim subset. admin.go is byte-identical at the recovery pin
# and the Gap 02 merge (same sha256), so the tuning gate pins that
# same whole-file hash.
check_file_at "$SRC_SHA_GAP02" "go/internal/query/admin/delta_union_completeness_read_admission.go" \
  "contracts/upstream/delta_union_completeness_read_admission.go" \
  "36ac5c63dd64c48a3985fd9645a70e3369908b2204d324eaae5ed0481e710c3e"

check_file_at "$SRC_SHA_GAP02" "go/internal/query/admin/delta_union_completeness_read_admission_contract_test.go" \
  "contracts/upstream/delta_union_completeness_read_admission_contract_test.go" \
  "dc6fdadc18352cec200e91f8c562e56345e336396c0b9aace560cfcd67996b61"

check_file_at "$SRC_SHA_GAP02" "go/internal/query/admin/reindex_progress_read_admission.go" \
  "contracts/upstream/reindex_progress_read_admission.go" \
  "dfdb95b3629722a488e60bf09f76c0dc02c7673905e1a5686ee1de908f520fba"

check_file_at "$SRC_SHA_GAP02" "go/internal/query/admin/reindex_progress_read_admission_contract_test.go" \
  "contracts/upstream/reindex_progress_read_admission_contract_test.go" \
  "990d173231923ae7f8789dccab296db9278c77b8cf1ca7cc4baf458d35c631a8"

check_file_at "$SRC_SHA_GAP02" "go/internal/query/admin/shared_projection_tuning_report_read_admission.go" \
  "contracts/upstream/shared_projection_tuning_report_read_admission.go" \
  "c1cc1ed0a10e2bdddc05fa9ca7b11304ed883a9ee5953aa6a227e0b983744d39"

check_file_at "$SRC_SHA_GAP02" "go/internal/query/admin/shared_projection_tuning_report_read_admission_contract_test.go" \
  "contracts/upstream/shared_projection_tuning_report_read_admission_contract_test.go" \
  "ef5b390bf4885124d246fefdeaa651f8a653ce6989a7785fa5952df635c541c4"

check_file_at "$SRC_SHA_GAP02" "go/internal/query/openapi/paths/status/delta_union_completeness.go" \
  "openapi/paths/status/delta_union_completeness.go" \
  "c6bfd5a9c663d3c584ecb247798e1407ceb729531d4d5a61098d9114dd582520"

check_file_at "$SRC_SHA_GAP02" "go/internal/query/openapi/paths/status/reindex_progress.go" \
  "openapi/paths/status/reindex_progress.go" \
  "d018e6d902873fd22a09a57959dd21124ce194b5c481ee93e9a262af06f10a88"

check_fragment_subset_named_at "$SRC_SHA_GAP02" "go/internal/query/openapi/paths/status/admin.go" \
  "openapi/paths/status/admin_tuning_report.go" \
  "26c7669744ebb6b29a18bb9d95c7186e24785421afb8d733d9d9a3a246a78eb9" \
  "AdminTuningReport"

# Gap 04 Compare pin (satellite eshu-hq/eshu-api#72, core
# eshu-hq/eshu#10425 merge 3870181f): the versioned admission pin
# (unwired-seam style — version constant + frozen method-set
# contract test), recorded byte-identical satellite-side first. No
# termination rides this record: the Compare POST stays relay until
# the termination lane lands.
check_file_at "$SRC_SHA_COMPARE" "go/internal/query/compare/read_admission_contract_test.go" \
  "contracts/upstream/compare_read_admission_contract_test.go" \
  "0a394ea8c3230c1bc978bc6c4f86d15b058805a607e3d63d60ec9ea6f0521b1a"

# O2/O6 six pins (satellite eshu-hq/eshu-api#72, core origin/main
# 14a5bf8a52): the versioned admission pins (unwired-seam style —
# version constant + frozen method-set contract test per pin),
# recorded byte-identical satellite-side first. No termination rides
# this record: every leg stays on its current relay or tier-C lane
# until its termination lane lands.
check_file_at "$SRC_SHA_O2O6" "go/internal/query/auth/audit_append_admission_contract_test.go" \
  "contracts/upstream/audit_append_admission_contract_test.go" \
  "268a736ea8309f59244353f2f321192c3abbbd5a9b312d41c0e8558dc7c63581"

check_file_at "$SRC_SHA_O2O6" "go/internal/query/auth/read_admission_contract_test.go" \
  "contracts/upstream/sign_in_policy_read_admission_contract_test.go" \
  "f9ec1530119e00121ea313abfa29e679897867e806cf0161faa2a7f86e763eea"

check_file_at "$SRC_SHA_O2O6" "go/internal/query/metrics/read_admission_contract_test.go" \
  "contracts/upstream/metrics_read_admission_contract_test.go" \
  "c794f47ff10e15bfe8a5476516c6cb3c77b635b39d4a829bd3551f6c7e7301bd"

check_file_at "$SRC_SHA_O2O6" "go/internal/query/semanticsearch/read_admission_contract_test.go" \
  "contracts/upstream/semantic_search_read_admission_contract_test.go" \
  "63dcc3bbb35945cd65968acce51a30c164971a73680ce941cf351cb803767766"

check_file_at "$SRC_SHA_O2O6" "go/internal/query/terraform/drift/read_admission_contract_test.go" \
  "contracts/upstream/terraform_drift_read_admission_contract_test.go" \
  "964f85f275ce0fd7eda690200e679f2a0dd7115f7beb11a11b1da40124584c6c"

check_file_at "$SRC_SHA_O2O6" "go/internal/query/iac/read_admission_contract_test.go" \
  "contracts/upstream/iac_read_admission_contract_test.go" \
  "2454d431ed9f6f04cdd47d1e720d99b2a7dd9c4245ee6e2b05f6da236043b242"

# W3 supplemental IaCInventorySearchAdmission v0 pin
# (eshu-hq/eshu#10578): the resources candidate-page pin files are
# byte-identical to the supplemental merge, and every line of the
# wire-shapes excerpt appears verbatim in the pinned core file.
check_file_at "$SRC_SHA_W3SUPP" "go/internal/query/iac/search_admission.go" \
  "contracts/upstream/iac_search_admission.go" \
  "6eb951dee5a69224182904782ed7df9b951137b886ab355995a0e74d21214210"

check_file_at "$SRC_SHA_W3SUPP" "go/internal/query/iac/search_admission_contract_test.go" \
  "contracts/upstream/iac_search_admission_contract_test.go" \
  "f91dbfec5c0f87733962200309deb1c07252dfdae2e4930071118c7fd7859366"

check_file_at "$SRC_SHA_W3SUPP" "go/internal/query/iac/search_admission_external_test.go" \
  "contracts/upstream/iac_search_admission_external_test.go" \
  "8b2967a1e1ebc978ea4a2b4fb73b73e1c23bcb0e1d1bb806c4ad7c029aab12cb"

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

# D6 phase-2 extra-port Status legs (eshu-hq/eshu#10405): the operations
# board is a whole-file byte-identical port at the merge commit; the
# index-status spellings ride a line-verbatim subset of the pinned core
# routes file (routes.go is byte-identical between the D6 pin and the
# phase-2 merge, so the file hash recurs — the gate still anchors each
# fragment at the phase-2 merge for provenance).
check_file_at "$SRC_SHA_STATUS_EXTRA" "go/internal/query/openapi/paths/status/operations.go" \
  "openapi/paths/status/operations.go" \
  "77ed558672a1d1ad54671388721704c539df5567ae45118e7e16cce1c029b5f8"

check_fragment_subset_named_at "$SRC_SHA_STATUS_EXTRA" "go/internal/query/openapi/paths/status/routes.go" \
  "openapi/paths/status/status_extra_reads.go" \
  "24a57b35c675cb33ad5011343a0aeb68d49e9d05d7645f397de2d88e8e5afa61" \
  "StatusExtraReads"

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

# Wave-2 Content wave (Decision 5(c)): the two schemas the lane
# entries reference, ported line-verbatim into the shared components
# block — FileContent and EntityContent from components.go.
check_content_components_at() {
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
  check_block '"FileContent": {' "$SRC_SHA_WAVE2_CONTENT" "go/internal/query/openapi/components.go" \
    || fail=1
  check_block '"EntityContent": {' "$SRC_SHA_WAVE2_CONTENT" "go/internal/query/openapi/components.go" \
    || fail=1
  # W4 search schemas the ported entries reference (satellite
  # eshu-hq/eshu-api#111): the entity-search response with its rank
  # walk, and the rank-cursor conflict response. Each dest block must
  # appear verbatim in its pinned core source at the W4 base.
  check_block '"EntityContentSearchResponse": {' "$SRC_SHA_W4" "go/internal/query/openapi/components_entity_search.go" \
    || fail=1
  check_block '"RankWalk": {' "$SRC_SHA_W4" "go/internal/query/openapi/components_rank_walk.go" \
    || fail=1
  check_block '"RankCursorConflict": {' "$SRC_SHA_W4" "go/internal/query/openapi/components_responses.go" \
    || fail=1
  if [[ "$fail" != "0" ]]; then
    exit 1
  fi
  echo "OK: Content components lines all verbatim in pinned component sources"
}

check_content_components_at

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

# D9 OIDC-login wave: the same excerpt re-anchored at the fresh wave
# SHA (byte-identical sources — re-anchor, no content change). The
# GitHub/SAML excerpts stay at their stale anchors until their own
# sequential lanes.
check_excerpt_at "$SRC_SHA_D9" "contracts/upstream/oidc-login-wire-shapes.txt" \
  "go/internal/query/oidc_login_handler.go" \
  "go/cmd/api/oidc_login.go"

# D9 GitHub-login wave: the same excerpt re-anchored at the fresh wave
# SHA (byte-identical sources — re-anchor, no content change). The SAML
# excerpt stays at its stale anchor until its own sequential lane.
check_excerpt_at "$SRC_SHA_D9_GITHUB" "contracts/upstream/github-login-wire-shapes.txt" \
  "go/internal/query/github_login_handler.go" \
  "go/cmd/api/github_login.go"

check_excerpt_at "$SRC_SHA_AUTHLOGIN" "contracts/upstream/github-login-wire-shapes.txt" \
  "go/internal/query/github_login_handler.go" \
  "go/cmd/api/github_login.go"

check_excerpt_at "$SRC_SHA_SAMLLOGIN" "contracts/upstream/saml-login-wire-shapes.txt" \
  "go/internal/query/saml_admission.go" \
  "go/internal/query/saml_handler.go"

# D9 SAML-login wave: the same excerpt re-anchored at the fresh wave
# SHA (byte-identical sources — re-anchor, no content change).
check_excerpt_at "$SRC_SHA_D9_SAML" "contracts/upstream/saml-login-wire-shapes.txt" \
  "go/internal/query/saml_admission.go" \
  "go/internal/query/saml_handler.go"

check_excerpt_at "$SRC_SHA_RECOVERY" "contracts/upstream/recovery-wire-shapes.txt" \
  "go/internal/query/admin/recovery_admission.go" \
  "go/internal/query/admin/handler.go"

# Retire-unclaimed writer pin (eshu-hq/eshu#10583): the admission shape
# excerpt, line-verbatim from the pin, handler, and mount sources at
# the merge commit.
check_excerpt_at "$SRC_SHA_RETIREUNCLAIMED" "contracts/upstream/retire-unclaimed-wire-shapes.txt" \
  "go/internal/query/admin/retire_unclaimed_admission.go" \
  "go/internal/query/admin/retire_unclaimed.go" \
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

check_excerpt_at "$SRC_SHA_GAP02" "contracts/upstream/admin-trio-read-wire-shapes.txt" \
  "go/internal/query/admin/delta_union_completeness_read_admission.go" \
  "go/internal/query/admin/reindex_progress_read_admission.go" \
  "go/internal/query/admin/shared_projection_tuning_report_read_admission.go" \
  "go/internal/query/admin/handler.go"

# Gap 04 Compare pin (eshu-hq/eshu#10425): the admission shape
# excerpt, line-verbatim from the pin file at the merge commit.
check_excerpt_at "$SRC_SHA_COMPARE" "contracts/upstream/compare-read-wire-shapes.txt" \
  "go/internal/query/compare/read_admission.go"

# O2/O6 six pins (core origin/main 14a5bf8a52): the admission shape
# excerpts, line-verbatim from the pin files at the record commit.
check_excerpt_at "$SRC_SHA_O2O6" "contracts/upstream/audit-append-wire-shapes.txt" \
  "go/internal/query/auth/audit_append_admission.go"

check_excerpt_at "$SRC_SHA_O2O6" "contracts/upstream/sign-in-policy-read-wire-shapes.txt" \
  "go/internal/query/auth/read_admission.go"

check_excerpt_at "$SRC_SHA_O2O6" "contracts/upstream/metrics-read-wire-shapes.txt" \
  "go/internal/query/metrics/read_admission.go"

check_excerpt_at "$SRC_SHA_O2O6" "contracts/upstream/semantic-search-read-wire-shapes.txt" \
  "go/internal/query/semanticsearch/read_admission.go"

check_excerpt_at "$SRC_SHA_O2O6" "contracts/upstream/terraform-drift-read-wire-shapes.txt" \
  "go/internal/query/terraform/drift/read_admission.go"

check_excerpt_at "$SRC_SHA_O2O6" "contracts/upstream/iac-read-wire-shapes.txt" \
  "go/internal/query/iac/read_admission.go"

check_excerpt_at "$SRC_SHA_D6" "contracts/upstream/status-read-wire-shapes.txt" \
  "go/internal/status/read_admission.go" \
  "go/internal/status/report.go"

check_excerpt_at "$SRC_SHA_D6" "contracts/upstream/supply-chain-read-wire-shapes.txt" \
  "go/internal/query/supply/chain/supply_read_admission.go"

# D6 phase-2 extra-port Status pin (eshu-hq/eshu#10405): the admission
# shape excerpt, line-verbatim from the pin file at the merge commit.
check_excerpt_at "$SRC_SHA_STATUS_EXTRA" "contracts/upstream/status-extra-read-wire-shapes.txt" \
  "go/internal/query/status_extra_read_admission.go"

# Wave-1 Freshness pin (eshu-hq/eshu#10494): the admission shape
# excerpt, line-verbatim from the pin file at the merge commit — the
# version const stanza plus the three re-declared method signatures.
check_excerpt_at "$SRC_SHA_WAVE1" "contracts/upstream/freshness-read-wire-shapes.txt" \
  "go/internal/query/freshness/read_admission.go"

# Wave-2 Content point-read pin (eshu-hq/eshu#10504): the admission
# shape excerpt, line-verbatim from the pin file at the merge commit —
# the version const stanza plus the three re-declared method
# signatures.
check_excerpt_at "$SRC_SHA_WAVE2_CONTENT" "contracts/upstream/content-point-read-wire-shapes.txt" \
  "go/internal/query/content_read_admission.go"

# W6a Code-flow pin (eshu-hq/eshu#10552): the admission shape excerpt,
# line-verbatim from the pin file at the merge commit — the version
# const stanza plus the one re-declared method signature.
check_excerpt_at "$SRC_SHA_W6A" "contracts/upstream/code-flow-read-wire-shapes.txt" \
  "go/internal/query/codequery/code_flow_read_admission.go"
# W4 search pins (eshu-hq/eshu#10540, recorded at the verified HEAD):
# the two minted v0 pins, byte-identical to the record base. The pins
# were minted at 330828359a; the content pin carries a post-merge
# instruments delta at the record base (constructor gains
# *telemetry.Instruments, recorded in contracts/
# content-search-read-admission.md), disclosed not silently assumed.
check_file_at "$SRC_SHA_W4" "contracts/content-search-read-admission.md" \
  "contracts/content-search-read-admission.md" \
  "2c955bdc2c95e079928e4b18189411e06fc11db9ce3dddd1b069c396ff922d3f"

check_file_at "$SRC_SHA_W4" "contracts/code-search-read-admission.md" \
  "contracts/code-search-read-admission.md" \
  "6a41f16e5f7705c51153956cbda903b196426a349e10fd40ceea0aec363b7ef0"

check_file_at "$SRC_SHA_W4" "go/internal/query/content_search_read_admission_contract_test.go" \
  "contracts/upstream/content_search_read_admission_contract_test.go" \
  "6a5eb7e63620dc552c6ef666341acad8645ec73f77a6039ae1db98f2e053e711"

check_file_at "$SRC_SHA_W4" "go/internal/query/code_search_read_admission_contract_test.go" \
  "contracts/upstream/code_search_read_admission_contract_test.go" \
  "b25093f073a59b16a423d716fc1235b461dd995c51b8ed0ce217ee7060e71b79"

# W4 search-pin admission shape excerpts, line-verbatim from the pin
# files at the record base — the version const stanza plus the
# re-declared method signatures (10 content, 9 code).
check_excerpt_at "$SRC_SHA_W4" "contracts/upstream/content-search-read-wire-shapes.txt" \
  "go/internal/query/content_search_read_admission.go"

check_excerpt_at "$SRC_SHA_W4" "contracts/upstream/code-search-read-wire-shapes.txt" \
  "go/internal/query/code_search_read_admission.go"

# W8a new pins (satellite eshu-hq/eshu-api#100): the admission shape
# excerpts, line-verbatim from the pin files at the W8a core HEAD.
check_excerpt_at "$SRC_SHA_W8A" "contracts/upstream/secretsiam-read-wire-shapes.txt" \
  "go/internal/query/secrets/read_admission.go"

check_excerpt_at "$SRC_SHA_W8A" "contracts/upstream/status-governance-read-wire-shapes.txt" \
  "go/internal/query/status_governance_read_admission.go"

check_excerpt_at "$SRC_SHA_W8A" "contracts/upstream/status-hosted-readiness-read-wire-shapes.txt" \
  "go/internal/query/status_hosted_readiness_read_admission.go"

# W8a re-verification at the W8a core HEAD (satellite
# eshu-hq/eshu-api#100, D13 tracker eshu-hq/eshu#10491): the five
# already-recorded pins plus StatusReadAdmission v0 and Compare prove
# byte-identity again at e504e328a7 in-lane — zero commits touch any
# pin file between its merge and the W8a HEAD, and these lines gate
# that standing in CI instead of asserting it.
check_excerpt_at "$SRC_SHA_W8A" "contracts/upstream/iac-read-wire-shapes.txt" \
  "go/internal/query/iac/read_admission.go"

# W3 supplemental IaCInventorySearchAdmission v0 pin
# (eshu-hq/eshu#10578): every line of the supplemental wire-shapes
# excerpt appears verbatim in the pinned core file.
check_excerpt_at "$SRC_SHA_W3SUPP" "contracts/upstream/iac-search-wire-shapes.txt" \
  "go/internal/query/iac/search_admission.go"

check_excerpt_at "$SRC_SHA_W8A" "contracts/upstream/metrics-read-wire-shapes.txt" \
  "go/internal/query/metrics/read_admission.go"

check_excerpt_at "$SRC_SHA_W8A" "contracts/upstream/semantic-search-read-wire-shapes.txt" \
  "go/internal/query/semanticsearch/read_admission.go"

check_excerpt_at "$SRC_SHA_W8A" "contracts/upstream/terraform-drift-read-wire-shapes.txt" \
  "go/internal/query/terraform/drift/read_admission.go"

check_excerpt_at "$SRC_SHA_W8A" "contracts/upstream/compare-read-wire-shapes.txt" \
  "go/internal/query/compare/read_admission.go"

check_excerpt_at "$SRC_SHA_W8A" "contracts/upstream/status-read-wire-shapes.txt" \
  "go/internal/status/read_admission.go" \
  "go/internal/status/report.go"

# Satellite Wave-4 Drift termination (eshu-hq/eshu-api#117): the
# CloudRuntimeDriftReadAdmission v0 pin, minted core-side W8b tier-1
# (unwired, arbiter-approved). The vendored contract test is
# byte-identical at the mint commit; the wire-shapes excerpt carries
# the version const stanza plus the two re-declared method signatures,
# line-verbatim from the pin file.
check_file_at "$SRC_SHA_DRIFT" "go/internal/query/cloud_runtime_drift_read_admission_contract_test.go" \
  "contracts/upstream/cloud_runtime_drift_read_admission_contract_test.go" \
  "89f313bdbe8ed6073d2ee68cfd29dc16b4931efb58a8ae06c965b5774f24ce77"

check_excerpt_at "$SRC_SHA_DRIFT" "contracts/upstream/cloud-runtime-drift-read-wire-shapes.txt" \
  "go/internal/query/cloud_runtime_drift_read_admission.go"

# Satellite Wave-5 dead-route termination (eshu-hq/eshu-api#119): the
# IaCDeadContentAdmission v0 dual pin, minted core-side W8b tier-1
# (unwired, arbiter-approved). The vendored contract test and external
# servability test are byte-identical at the merge commit; the
# wire-shapes excerpt carries the version const stanza, the
# four-method interface, the ContentStore mirror assertion, and the
# surrounding contract comment, line-verbatim from the pin file.
check_file_at "$SRC_SHA_DEAD" "go/internal/query/iac/dead_content_admission.go" \
  "contracts/upstream/iac_dead_content_admission.go" \
  "a3da56bbe6f60803a5c372b2918f537d80dc1cb53e7fc6177e034a4a2f3ab0cd"

check_file_at "$SRC_SHA_DEAD" "go/internal/query/iac/dead_content_admission_contract_test.go" \
  "contracts/upstream/iac_dead_content_admission_contract_test.go" \
  "adba362c286cb18bd4e6a37fd330d4b7c4e3baed553fb0249949b887c5136118"

check_file_at "$SRC_SHA_DEAD" "go/internal/query/iac/dead_content_admission_external_test.go" \
  "contracts/upstream/iac_dead_content_admission_external_test.go" \
  "0ef8b3a882c2476ab7da11a679dd6b055c23f8d2810664cc36e8db161154ac11"

check_excerpt_at "$SRC_SHA_DEAD" "contracts/upstream/iac-dead-content-wire-shapes.txt" \
  "go/internal/query/iac/dead_content_admission.go"

# Satellite Wave-7 G-a termination (eshu-hq/eshu-api#121): the
# CodeGraphReadAdmission v0 pin, minted core-side (unwired,
# arbiter-approved SPLIT). The vendored pin and contract test are
# byte-identical at the merge commit; the wire-shapes excerpt carries
# the version const stanza, the in-scope route list, the 3-method
# interface, the GraphQuery/ContentStore mirror assertion, and the
# surrounding contract comment, line-verbatim from the pin file.
check_file_at "$SRC_SHA_GRAPH" "go/internal/query/codequery/code_graph_read_admission.go" \
  "contracts/upstream/code_graph_read_admission.go" \
  "1ef71affe095a4fa9ca4732f6cff28601e159b6e26c21c1acb0ae864511fbeac"

check_file_at "$SRC_SHA_GRAPH" "go/internal/query/codequery/code_graph_read_admission_contract_test.go" \
  "contracts/upstream/code_graph_read_admission_contract_test.go" \
  "d0a621c8d1d4ca4d809a5ce6acd9512cca651ee6efe9497b884f754e6de403ae"

check_excerpt_at "$SRC_SHA_GRAPH" "contracts/upstream/code-graph-read-wire-shapes.txt" \
  "go/internal/query/codequery/code_graph_read_admission.go"

# Satellite divergence termination (eshu-hq/eshu-api#125): the
# CodeDivergenceReadAdmission v0 pin, minted core-side (unwired,
# arbiter-approved SUPPLEMENTAL-PIN-OWED). The vendored pin and contract
# test are byte-identical at the merge commit; the wire-shapes excerpt
# carries the version const stanza, the in-scope route list, the
# 6-method interface, the divergenceStore/GraphQuery mirror assertion,
# and the surrounding contract comment, line-verbatim from the pin file.
check_file_at "$SRC_SHA_DIVERGENCE" "go/internal/query/codequery/code_divergence_read_admission.go" \
  "contracts/upstream/code_divergence_read_admission.go" \
  "38ea53daa80291b1adc5a63c01e98a87310c7ebfb6a22679cb3323593086fd50"

check_file_at "$SRC_SHA_DIVERGENCE" "go/internal/query/codequery/code_divergence_read_admission_contract_test.go" \
  "contracts/upstream/code_divergence_read_admission_contract_test.go" \
  "869d84a35deb8f02b508dab7f5370294ca263a11cf4b44201862e3f991c4c759"

check_excerpt_at "$SRC_SHA_DIVERGENCE" "contracts/upstream/code-divergence-read-wire-shapes.txt" \
  "go/internal/query/codequery/code_divergence_read_admission.go"

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
grep -qF 'RetireUnclaimedAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/retire-unclaimed-wire-shapes.txt"
grep -qF 'AdminReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/admin-read-wire-shapes.txt"
grep -qF 'AdminIdentityReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/admin-identity-read-wire-shapes.txt"
grep -qF 'GovernanceAuditReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/admin-identity-read-wire-shapes.txt"
grep -qF 'TerminalFailureReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/terminal-failure-read-wire-shapes.txt"
grep -qF 'DeltaUnionCompletenessReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/admin-trio-read-wire-shapes.txt"
grep -qF 'ReindexProgressReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/admin-trio-read-wire-shapes.txt"
grep -qF 'SharedProjectionTuningReportContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/admin-trio-read-wire-shapes.txt"
grep -qF 'CompareReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/compare-read-wire-shapes.txt"
grep -qF 'AuditAppendAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/audit-append-wire-shapes.txt"
grep -qF 'SignInPolicyReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/sign-in-policy-read-wire-shapes.txt"
grep -qF 'MetricsReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/metrics-read-wire-shapes.txt"
grep -qF 'SemanticSearchReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/semantic-search-read-wire-shapes.txt"
grep -qF 'TerraformDriftReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/terraform-drift-read-wire-shapes.txt"
grep -qF 'IaCReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/iac-read-wire-shapes.txt"
grep -qF 'FreshnessReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/freshness-read-wire-shapes.txt"
grep -qF 'ContentPointReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/content-point-read-wire-shapes.txt"
grep -qF 'CodeFlowReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/code-flow-read-wire-shapes.txt"
grep -qF 'ContentSearchReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/content-search-read-wire-shapes.txt"
grep -qF 'CodeSearchReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/code-search-read-wire-shapes.txt"
grep -qF 'CloudRuntimeDriftReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/cloud-runtime-drift-read-wire-shapes.txt"
grep -qF 'IaCDeadContentAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/iac-dead-content-wire-shapes.txt"
grep -qF 'CodeGraphReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/code-graph-read-wire-shapes.txt"
grep -qF 'CodeDivergenceReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/code-divergence-read-wire-shapes.txt"
grep -qF 'StatusReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/status-read-wire-shapes.txt"
grep -qF 'SupplyChainReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/supply-chain-read-wire-shapes.txt"
grep -qF 'StatusExtraReadAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/status-extra-read-wire-shapes.txt"
grep -qF 'SessionResolverAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/session_admission.go"
grep -qF 'BrowserSessionAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/session_admission.go"
grep -qF 'LocalIdentityAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/session_admission.go"
grep -qF 'APITokenAdmissionContractVersion = "v0"' "${REPO_ROOT}/contracts/upstream/session_admission.go"
echo "OK: version pins v0 present"

# D9 SAML-login wave: no dest-side client on the lane (arbiter ruling
# decision 3, binding). api/d9saml.go must name no dest-side client
# type, interface, or import — no store, no provider config, no
# verifier, no keyring, no client library — and must contain no form or
# body read: the ACS form crosses opaquely inside the shared streaming
# forward. Fails closed on any match (the in-suite source tests
# TestD9SAMLNative_NoDestSideFormReads and
# TestD9SAMLNative_NoDestSideClientSymbols prove the same list).
for symbol in SAMLStore SAMLProviderConfig ServiceProviderConfig samlauth secretcrypto keyring; do
  if grep -qF "$symbol" "${REPO_ROOT}/api/d9saml.go"; then
    echo "FAIL: api/d9saml.go names ${symbol}: no dest-side SAML client on this lane" >&2
    exit 1
  fi
done
for call in ParseForm FormValue PostForm MultipartForm 'ReadAll(' MaxBytesReader; do
  if grep -qF "$call" "${REPO_ROOT}/api/d9saml.go"; then
    echo "FAIL: api/d9saml.go contains ${call}: no dest-side form/body read on this lane" >&2
    exit 1
  fi
done
echo "OK: api/d9saml.go carries no dest-side client or form/body read"

# SAML-triple termination: no dest-side client on the lane (arbiter
# ruling decision 3, binding). api/saml_terminate.go must name no
# dest-side client type, interface, or import — no store, no provider
# config, no verifier, no keyring, no client library — and above all
# never GetSAMLProvider: that store method returns the SP key material
# (service-provider private key, IdP metadata) that must never leave
# core, so the dest-side metadata call is deliberately named
# FetchSAMLMetadata instead. The lane must also contain no form parse
# and no provider extraction: the ACS form crosses as bounded bytes
# (the ReadAll-over-LimitReader size bound is the one deliberate
# exception — size-bounding is not parsing) and the provider path
# value crosses whole, never segmented. Fails closed on any match
# (the in-suite source test TestSAMLTerm_LaneFileHygiene proves the
# same list plus the log/store/lock bans).
for symbol in GetSAMLProvider SAMLStore SAMLProviderConfig ServiceProviderConfig samlauth secretcrypto keyring; do
  if grep -qF "$symbol" "${REPO_ROOT}/api/saml_terminate.go"; then
    echo "FAIL: api/saml_terminate.go names ${symbol}: no dest-side SAML client on this lane" >&2
    exit 1
  fi
done
for call in ParseForm FormValue PostForm MultipartForm MultipartReader MaxBytesReader 'PathValue(' '.RequestURI'; do
  if grep -qF "$call" "${REPO_ROOT}/api/saml_terminate.go"; then
    echo "FAIL: api/saml_terminate.go contains ${call}: no dest-side form parse or path extraction on this lane" >&2
    exit 1
  fi
done
echo "OK: api/saml_terminate.go carries no dest-side client or form parse"

# Wave-1 Freshness lane: no dest-side store on the lane (arbiter
# Wave-1 ruling, sealed 3-method admission). api/freshness_terminate.go
# and api/freshness_wave.go must name no dest-side store, reader port,
# or ledger/summary/snapshot symbol — the pin's store, readers, ledger
# flag, summary reader, and snapshot legs all stay core-side, and the
# lane forwards the filter queries opaquely (no query, form, or path
# parse). Fails closed on any match (the in-suite source test
# TestFreshnessTerm_LaneFileHygiene proves the same list).
for lane in freshness_terminate freshness_wave; do
  for symbol in postgres StatusStore ReadStore LedgerReadEnabled WithSummaryReader SummaryReader Snapshot 'database/sql' sql.DB sql.Open; do
    if grep -qF "$symbol" "${REPO_ROOT}/api/${lane}.go"; then
      echo "FAIL: api/${lane}.go names ${symbol}: no dest-side store on this lane" >&2
      exit 1
    fi
  done
  for call in '.Query(' ParseForm FormValue PathValue '.RequestURI'; do
    if grep -qF "$call" "${REPO_ROOT}/api/${lane}.go"; then
      echo "FAIL: api/${lane}.go contains ${call}: no dest-side query/form/path parse on this lane" >&2
      exit 1
    fi
  done
done
echo "OK: Wave-1 Freshness lane files carry no dest-side store or parse"

# Wave-2 Content lane: no dest-side store on the lane (arbiter Wave-2
# ruling, sealed 3-method admission). api/content_terminate.go and
# api/content_wave.go must name no dest-side store, reader port, search
# or list leg, or scoped-entity symbol — the pin's store, readers, and
# out-of-pin legs all stay core-side, and the lane forwards the request
# bodies opaquely (no query, form, path, or body parse). Fails closed
# on any match (the in-suite source test
# TestContentPointReadsTerm_LaneFileHygiene proves the same list).
for lane in content_terminate content_wave; do
  for symbol in postgres ContentStore ContentReader ReadStore SearchFileContent SearchEntityContent ListRepoFiles ListRepoEntities GetEntityContentInRepositories 'database/sql' sql.DB sql.Open; do
    if grep -qF "$symbol" "${REPO_ROOT}/api/${lane}.go"; then
      echo "FAIL: api/${lane}.go names ${symbol}: no dest-side store on this lane" >&2
      exit 1
    fi
  done
  for call in '.Query(' ParseForm FormValue PathValue '.RequestURI' '"repo_id"' '"relative_path"' '"entity_id"' '"start_line"' '"end_line"'; do
    if grep -qF "$call" "${REPO_ROOT}/api/${lane}.go"; then
      echo "FAIL: api/${lane}.go contains ${call}: no dest-side query/form/path/body parse on this lane" >&2
      exit 1
    fi
  done
done
echo "OK: Wave-2 Content lane files carry no dest-side store or parse"

# W4 Content search lane (satellite eshu-hq/eshu-api#111): no dest-side
# store on the lane (arbiter W4 ruling, sealed 10-method admission).
# api/content_search_terminate.go and api/content_search_wave.go must
# name no dest-side store, reader port, or search/list leg symbol — the
# pin's store and readers all stay core-side, and the lane forwards the
# request bodies opaquely (no query, form, path, or body parse). Fails
# closed on any match (the in-suite source test
# TestContentSearchTerm_LaneFileHygiene proves the same list).
for lane in content_search_terminate content_search_wave; do
  for symbol in postgres ContentStore ContentReader ReadStore SearchFileContent SearchEntityContent ListRepoFiles ListRepoEntities GetEntityContentInRepositories 'database/sql' sql.DB sql.Open; do
    if grep -qF "$symbol" "${REPO_ROOT}/api/${lane}.go"; then
      echo "FAIL: api/${lane}.go names ${symbol}: no dest-side store on this lane" >&2
      exit 1
    fi
  done
  for call in '.Query(' ParseForm FormValue PathValue '.RequestURI' '"repo_id"' '"relative_path"' '"entity_id"' '"start_line"' '"end_line"'; do
    if grep -qF "$call" "${REPO_ROOT}/api/${lane}.go"; then
      echo "FAIL: api/${lane}.go contains ${call}: no dest-side query/form/path/body parse on this lane" >&2
      exit 1
    fi
  done
done
echo "OK: W4 Content search lane files carry no dest-side store or parse"

# W4 Code search lane (satellite eshu-hq/eshu-api#113): no dest-side
# store on the lane (arbiter W4 ruling, sealed 9-method admission, no
# transport constructor by design). api/code_search_terminate.go and
# api/code_search_wave.go must name no dest-side store, reader port,
# graph/content leg, or out-of-pin leg symbol — the pin's graph reader,
# content reader, and out-of-pin legs (exact-name twins, file legs,
# ledger/summary/snapshot, point reads) all stay core-side, and the
# lane forwards the request bodies opaquely (no query, form, path, or
# body parse). Fails closed on any match (the in-suite source test
# TestCodeSearchTerm_LaneFileHygiene proves the same list).
for lane in code_search_terminate code_search_wave; do
  for symbol in postgres GraphQuery ContentReader ReadStore RunSingle SearchEntityNames SearchSymbols SearchCodeCandidates SearchEntitiesByName SearchEntityContent ListRepoEntitiesByIDs 'database/sql' sql.DB sql.Open; do
    if grep -qF "$symbol" "${REPO_ROOT}/api/${lane}.go"; then
      echo "FAIL: api/${lane}.go names ${symbol}: no dest-side store on this lane" >&2
      exit 1
    fi
  done
  for call in '.Query(' ParseForm FormValue PathValue '.RequestURI' '"query"' '"repo_id"' '"language"' '"limit"' '"offset"' '"exact"' '"rank_cursor"' '"symbol"' '"match_mode"'; do
    if grep -qF "$call" "${REPO_ROOT}/api/${lane}.go"; then
      echo "FAIL: api/${lane}.go contains ${call}: no dest-side query/form/path/body parse on this lane" >&2
      exit 1
    fi
  done
done
echo "OK: W4 Code search lane files carry no dest-side store or parse"

# Satellite Wave-4 Drift termination (eshu-hq/eshu-api#117): no
# dest-side store on the lane (arbiter W8b ruling, sealed 2-method
# admission, no transport constructor by design).
# api/drift_terminate.go and api/drift_wave.go must name no dest-side
# store, reader port, or drift leg symbol — the pin's multi-cloud
# drift store and the out-of-pin graph/content/snapshot/ledger legs
# all stay core-side, and the lane forwards the request bodies and
# packet queries opaquely (no query, form, path, or body parse). Fails
# closed on any match (the in-suite source test
# TestDriftTerm_LaneFileHygiene proves the same list).
for lane in drift_terminate drift_wave; do
  for symbol in postgres MultiCloudRuntimeDrift CloudRuntimeDriftReadAdmission ListActiveMultiCloudRuntimeDriftFindings CountActiveMultiCloudRuntimeDriftFindings ReadStore GraphQuery ContentReader RunSingle 'database/sql' sql.DB sql.Open; do
    if grep -qF "$symbol" "${REPO_ROOT}/api/${lane}.go"; then
      echo "FAIL: api/${lane}.go names ${symbol}: no dest-side store on this lane" >&2
      exit 1
    fi
  done
  for call in '.Query(' ParseForm FormValue PathValue '.RequestURI' '"scope_id"' '"provider"' '"limit"' '"offset"' '"cloud_resource_uid"' '"finding_kinds"'; do
    if grep -qF "$call" "${REPO_ROOT}/api/${lane}.go"; then
      echo "FAIL: api/${lane}.go contains ${call}: no dest-side query/form/path/body parse on this lane" >&2
      exit 1
    fi
  done
done
echo "OK: Wave-4 Drift lane files carry no dest-side store or parse"

# Satellite Wave-5 dead-route termination (eshu-hq/eshu-api#119): no
# dest-side store, port, or parse of Content on the lane (arbiter W8b
# ruling, sealed 4-method supplemental admission). api/iac.go must name
# no dest-side Content symbol — the pin's four ContentStore reads and
# the out-of-pin graph/inventory/management/selector/reachability legs
# all stay core-side, and the lane forwards the request bodies
# opaquely (no query, form, path, or body-key parse). Pin names in
# comments stay allowed (the lane record names its pins); only
# executable coupling fails. Fails closed on any match (the in-suite
# source test TestIaCLaneFileHygiene proves the same list).
for symbol in ContentStore ContentReader ListRepositories MatchRepositories ListRepoFiles GetFileContent RepositoryCatalogEntry FileContent querycontract postgres 'database/sql' sql.DB sql.Open; do
  if grep -qF "$symbol" "${REPO_ROOT}/api/iac.go"; then
    echo "FAIL: api/iac.go names ${symbol}: no dest-side Content on this lane" >&2
    exit 1
  fi
done
for call in '.Query(' ParseForm FormValue PathValue '.RequestURI' '"repo_id"' '"repo_ids"' '"families"' '"limit"' '"offset"' 'slog.' 'tracing.' 'span.' 'json.Marshal' 'sync.' 'Mutex'; do
  if grep -qF "$call" "${REPO_ROOT}/api/iac.go"; then
    echo "FAIL: api/iac.go contains ${call}: no dest-side query/form/path/body-key parse on this lane" >&2
    exit 1
  fi
done
echo "OK: Wave-5 dead-route lane file carries no dest-side Content or parse"

# Satellite Wave-7 G-a termination (eshu-hq/eshu-api#121): no
# dest-side store, port, or parse of the graph/content reads on the
# lane (arbiter Wave-7 ruling, sealed 3-method admission, no transport
# constructor by design). api/code_graph_terminate.go and
# api/code_graph_wave.go must name no dest-side store, reader port,
# graph/content leg, or out-of-pin leg symbol — the pin's graph
# reader, content reader, and out-of-pin relationship/summary/ledger
# legs all stay core-side, and the lane forwards the request bodies
# opaquely (no query, form, path, or body-key parse). Fails closed on
# any match (the in-suite source test
# TestCodeGraphTerm_LaneFileHygiene proves the same list).
for lane in code_graph_terminate code_graph_wave; do
  for symbol in postgres CodeGraphReadAdmission GraphQuery ContentStore ContentReader ReadStore RunSingle SearchEntitiesByName 'database/sql' sql.DB sql.Open; do
    if grep -qF "$symbol" "${REPO_ROOT}/api/${lane}.go"; then
      echo "FAIL: api/${lane}.go names ${symbol}: no dest-side store on this lane" >&2
      exit 1
    fi
  done
  for call in '.Query(' ParseForm FormValue PathValue '.RequestURI' '"repo_id"' '"cross_repo"' '"start_repo_id"' '"end_repo_id"' '"max_depth"' '"max_paths"' '"metric_type"' '"limit"' '"offset"' '"method"' '"service_id"' '"service_name"'; do
    if grep -qF "$call" "${REPO_ROOT}/api/${lane}.go"; then
      echo "FAIL: api/${lane}.go contains ${call}: no dest-side query/form/path/body-key parse on this lane" >&2
      exit 1
    fi
  done
done
echo "OK: Wave-7 G-a lane files carry no dest-side store or parse"

# Satellite divergence termination (eshu-hq/eshu-api#125): no
# dest-side store, port, or parse of the divergence reads on the
# lane (arbiter SUPPLEMENTAL-PIN-OWED ruling, sealed 6-method
# admission, no transport constructor by design).
# api/divergence_terminate.go and api/divergence_wave.go must name no
# dest-side store, reader port, or out-of-pin leg symbol — the pin's
# graph reader, content reader, and out-of-pin search/list/summary/
# ledger legs all stay core-side, and the lane forwards the request
# bodies opaquely (no query, form, path, or body-key parse). Fails
# closed on any match (the in-suite source test
# TestDivergenceTerm_LaneFileHygiene proves the same list).
for lane in divergence_terminate divergence_wave; do
  for symbol in postgres CodeDivergenceReadAdmission GraphQuery ContentStore ContentReader ReadStore DivergenceGroupStats DivergenceMembers DriftedFinding 'database/sql' sql.DB sql.Open; do
    if grep -qF "$symbol" "${REPO_ROOT}/api/${lane}.go"; then
      echo "FAIL: api/${lane}.go names ${symbol}: no dest-side store on this lane" >&2
      exit 1
    fi
  done
  for call in '.Query(' ParseForm FormValue PathValue '.RequestURI' '"repo_id"' '"kind"' '"fingerprint"' '"limit"' '"offset"' '"include_tests"' '"top_per_kind"'; do
    if grep -qF "$call" "${REPO_ROOT}/api/${lane}.go"; then
      echo "FAIL: api/${lane}.go contains ${call}: no dest-side query/form/path/body-key parse on this lane" >&2
      exit 1
    fi
  done
done
echo "OK: divergence lane files carry no dest-side store or parse"

# W7-read admin sign-in-policy lane: no dest-side store on the lane
# (arbiter W7 ruling, sealed 1-method admission). api/
# signinpolicy_admin_terminate.go must name no dest-side store,
# reader port, or table symbol — the pin's store stays core-side,
# and the lane forwards the keyed call opaquely (no query, form,
# path, or credential parse; the tenant and the credential cross
# whole). Fails closed on any match (the in-suite source test
# TestAdminSignInPolicyTerm_LaneFileHygiene proves the same list).
for symbol in postgres SignInPolicyStore ReadStore IdentitySubjectStore 'database/sql' sql.DB sql.Open; do
  if grep -qF "$symbol" "${REPO_ROOT}/api/signinpolicy_admin_terminate.go"; then
    echo "FAIL: api/signinpolicy_admin_terminate.go names ${symbol}: no dest-side store on this lane" >&2
    exit 1
  fi
done
for call in '.Query(' ParseForm FormValue PathValue '.RequestURI' '"tenant_id"' '"Authorization"'; do
  if grep -qF "$call" "${REPO_ROOT}/api/signinpolicy_admin_terminate.go"; then
    echo "FAIL: api/signinpolicy_admin_terminate.go contains ${call}: no dest-side query/form/path/credential parse on this lane" >&2
    exit 1
  fi
done
echo "OK: W7-read admin sign-in-policy lane file carries no dest-side store or parse"

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
