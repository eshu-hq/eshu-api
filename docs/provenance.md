# Source provenance

Seed for `eshu-hq/eshu#8872` (part of `eshu-hq/eshu#6707`). Status:
preparation only; nothing here carries cutover authority.

## Pins

| Artifact | Pin |
| --- | --- |
| Source repository | `eshu-hq/eshu` |
| Source commit | `428790ab3ac0bff2eb28e50a26b3746a9b7dc8c5` (`origin/main` at the replay-ledger merge; covers the suppression, terminal-failure, sign-in, and replay-ledger admission seams) |
| Source commit (retirement, admin-identity) | `ebaf4dfa0de672851bbec7a427e28fdb41344380` (`origin/main` at the admin-identity merge; covers both seams, byte-identical since) |
| Source commit (setup) | `fd326f35c9fb497eec0abefad197a5d4b137b823` (merge of `eshu-hq/eshu#9853`; method set frozen at v0) |
| Source commit (provider-config) | `99381691b1ac49254e813e5a013cfb654d432491` (merge of `eshu-hq/eshu#9870`; mutation 5 + read 3 methods, frozen at v0) |
| Source commit (oidc-login, github-login) | `1e03478614fc54882b9616d37ea9103ea5ada6f4` (merge of `eshu-hq/eshu#9863`; 2 + 2 service methods, frozen at v0) |
| Source commit (saml-login) | `aac271ad78e328462dabc7518d5f7e5739bdc651` (merge of `eshu-hq/eshu#9897`; 5 store methods, frozen at v0, with the lister carve-out) |
| Source commit (recovery) | `7c60583996d9a3ccc46288f40a38f614a4f28ea8` (merge of `eshu-hq/eshu#10245`; RecoveryAdmission v0 over RecoveryService, frozen at v0, with the replay/reopen/refinalize/recover-generations Mount; openapi admin.go + recovery components at the same commit) |
| Source commit (D1 read pins) | `2c89a9d8b0a5efa1c4fbdd1d3e24de00012d1923` (merge of `eshu-hq/eshu#10259`; Admin, AdminIdentity, GovernanceAudit, and TerminalFailure read admissions v0, unwired-seam style: version constant + admission alias + frozen method-set test per pin) |
| Source commit (D2 Leaf B wave) | `17feac86e089a6da208fe6c93b5d89b8fb7ef3cc` (`origin/main` at the D2 lane base; wave fragments, doc-UI shells, and wave components entries, frozen at v0; the D4 Native-next fragments and components entries reuse this pin — every D4 file is byte-identical there and at `origin/main`, so no new source pin is minted) |
| Source commit (D6 read pins) | `6b66e6b23efe226970b4571c955aed1dc42f987a` (merge of `eshu-hq/eshu#10303`; Status and SupplyChain read admissions v0, unwired-seam style: version constant + admission alias + frozen method-set test per pin) |
| Source commit (D7 admin-four pins) | `19de7346d11a184bf88d8f307b5d7a9ced0a4657` (merge of `eshu-hq/eshu#10312`; Backfill, DeadLetter, Reindex, and Skip admissions v0, unwired-seam style: version constant + narrow port + frozen method-set test per pin; arbiter APPROVED-WITH-CONDITIONS) |
| Destination base | `a49341a8eb73ee620e97df4b88b5ddf93efc62c5` (README-only `main`) |
| `CoreReadContractsVersion` | `v0` |
| `SuppressionAdmissionContractVersion` | `v0` |
| `TerminalFailureAdmissionContractVersion` | `v0` |
| `ReplayLedgerAdmissionContractVersion` | `v0` |
| `SignInPolicyAdmissionContractVersion` | `v0` |
| `RetirementAdmissionContractVersion` | `v0` |
| `AdminIdentityAdmissionContractVersion` | `v0` |
| `SetupAdmissionContractVersion` | `v0` |
| `ProviderConfigMutationAdmissionContractVersion` | `v0` |
| `ProviderConfigReadAdmissionContractVersion` | `v0` |
| `OIDCLoginAdmissionContractVersion` | `v0` |
| `GitHubLoginAdmissionContractVersion` | `v0` |
| `SAMLLoginAdmissionContractVersion` | `v0` |
| `RecoveryAdmissionContractVersion` | `v0` |
| `AdminReadAdmissionContractVersion` | `v0` |
| `AdminIdentityReadAdmissionContractVersion` | `v0` |
| `GovernanceAuditReadAdmissionContractVersion` | `v0` |
| `TerminalFailureReadAdmissionContractVersion` | `v0` |
| `StatusReadAdmissionContractVersion` | `v0` |
| `SupplyChainReadAdmissionContractVersion` | `v0` |
| `BackfillAdmissionContractVersion` | `v0` |
| `DeadLetterAdmissionContractVersion` | `v0` |
| `ReindexAdmissionContractVersion` | `v0` |
| `SkipAdmissionContractVersion` | `v0` |

## Mapping

`contracts/upstream/` files are byte-exact copies of the pinned sources;
`scripts/check-parity.sh` re-fetches each source at the pinned commit and
compares bytes. The census and destination plan are referenced, not
copied: they keep evolving in the source until the extraction PR.

| Source path at pin | Destination | sha256 at pin | Treatment |
| --- | --- | --- | --- |
| `go/internal/query/querycontract/api_boundary.go` | `contracts/upstream/api_boundary.go` | `c5f3ab34bc6e8d43c119b9aa7028ba298fb7f15cc68ffd8386e63ad9c140d9a1` | verbatim |
| `go/internal/query/querycontract/api_boundary_test.go` | `contracts/upstream/api_boundary_test.go` | `346fd5fe3a40257812108e7d4fb72c3e683a28f0412e121781e8bcb9349a92b8` | verbatim |
| `go/internal/query/supply/chain/suppression_admission_contract_test.go` | `contracts/upstream/suppression_admission_contract_test.go` | `7f9348271464c27ac27afa7201874dc5e06fd0f2ff0cb096dff9b02fedd76ee3` | verbatim |
| `go/internal/storage/postgres/db/read_store_contract_test.go` | `contracts/upstream/read_store_contract_test.go` | `b4d021da1c406870555ab4d414f1e7962ddf187d3c996e8012f10ddd70cd3166` | verbatim |
| `go/internal/query/supply/chain/suppression_mutation.go` lines 44-82 | `contracts/upstream/suppression-wire-shapes.txt` | excerpt (line-presence at pin) | verbatim excerpt |
| `go/internal/query/admin/acknowledgement/admission_contract_test.go` | `contracts/upstream/terminal-failure_admission_contract_test.go` | `a76c88800bb0dc2f4e853632dfefde61fcc25648588ecb93036c296bb851ed47` | verbatim |
| `go/internal/query/admin/acknowledgement/admission.go` (terminal-failure block) + `handler.go` (`acknowledgeRequest`) | `contracts/upstream/terminal-failure-wire-shapes.txt` | excerpt (line-presence at pin) | verbatim excerpt |
| `go/internal/query/admin/acknowledgement/replay_ledger_admission_contract_test.go` | `contracts/upstream/replay_ledger_admission_contract_test.go` | `0a7139031b54ca4005f71498dfb43784e1c75264aa758c8f313462c9a9ff1ba7` | verbatim |
| `go/internal/query/admin/acknowledgement/admission.go` (ledger block) + `handler.go` (`Ledger`) | `contracts/upstream/replay-ledger-wire-shapes.txt` | excerpt (line-presence at pin) | verbatim excerpt |
| `go/internal/query/openapi/paths/status/collector_terminal_failure_acknowledgement.go` | `openapi/paths/status/collector_terminal_failure_acknowledgement.go` | `238401aab7e91208d466cd897beb7257d2be8f845ee4cc5025e35b15746e0121` | verbatim |
| `go/internal/query/sign_in_policy_admission_contract_test.go` | `contracts/upstream/sign_in_policy_admission_contract_test.go` | `3ce68b7ea519acf581acb70bb42929d03deed0de5c99e6312329078c71c72842` | verbatim |
| `go/internal/query/sign_in_policy_types.go` (`SignInPolicyUpdateRequest`, version, alias) | `contracts/upstream/sign-in-wire-shapes.txt` | excerpt (line-presence at pin) | verbatim excerpt |
| `go/internal/query/admin/retirement/admission_contract_test.go` at `ebaf4dfa0d` | `contracts/upstream/retirement_admission_contract_test.go` | `8ade12f13bd24786ee348c27807b223e24a685abe7d114b46abfeff1ebe926dd` | verbatim |
| `go/internal/query/admin/retirement/admission.go` (version, alias) + `handler.go` (`Triggerer`) at `ebaf4dfa0d` | `contracts/upstream/retirement-wire-shapes.txt` | excerpt (line-presence at pin) | verbatim excerpt |
| `go/internal/query/admin/identity/admission_contract_test.go` at `ebaf4dfa0d` | `contracts/upstream/admin_identity_admission_contract_test.go` | `1dcf3c4cb97170cbd0c3a33d65539e3a1472ff66381e2a9762e9a60c96826559` | verbatim |
| `go/internal/query/admin/identity/admission.go` (version, alias) + `types.go` (`MutationStore`) at `ebaf4dfa0d` | `contracts/upstream/admin-identity-wire-shapes.txt` | excerpt (line-presence at pin) | verbatim excerpt |
| `go/internal/query/setup_admission_contract_test.go` at `1b35b907ee` | `contracts/upstream/setup_admission_contract_test.go` | `b5dc9640b1683fd0cd259343483201ecd3c0c5a5d68262205fa65c3cb827a0ea` | verbatim |
| `go/internal/query/setup_types.go` (version, alias, `SetupStore`, input/owner/response shapes) at `1b35b907ee` | `contracts/upstream/setup-wire-shapes.txt` | excerpt (line-presence at pin) | verbatim excerpt |
| `go/internal/query/admin/provider/config/mutation_admission_contract_test.go` at `99381691` | `contracts/upstream/mutation_admission_contract_test.go` | `15bd3551a50b503919a065da7fa8e077a0ac7179b3f3a9e8160673592f4d0e3b` | verbatim |
| `go/internal/query/admin/provider/config/read_admission_contract_test.go` at `99381691` | `contracts/upstream/read_admission_contract_test.go` | `51d80f84961ea7b8e25e51c69b5c406b38f00ed77edfafd31c419450bfc4886d` | verbatim |
| `go/internal/query/admin/provider/config/admission.go` (versions, aliases) + `types.go` (`MutationStore`, `ReadStore`) + `mutations.go` (`Mount`) at `99381691` | `contracts/upstream/provider-config-wire-shapes.txt` | excerpt (line-presence at pin) | verbatim excerpt |
| `go/internal/query/oidc_login_admission_contract_test.go` at `1e034786` | `contracts/upstream/oidc_login_admission_contract_test.go` | `03342f7ad0d28aef72583ce21a7747fe360fe755367fef40660e58361f19d2c0` | verbatim |
| `go/internal/query/oidc_login_handler.go` (version, alias, `OIDCLoginService`, request/response shapes, `Mount`) + `go/cmd/api/oidc_login.go` (adapter, 503) at `1e034786` | `contracts/upstream/oidc-login-wire-shapes.txt` | excerpt (line-presence at pin) | verbatim excerpt |
| `go/internal/query/github_login_admission_contract_test.go` at `1e034786` | `contracts/upstream/github_login_admission_contract_test.go` | `f6a7fc9920176c76c8cf6e5ec223e881323564405b5931770d43db9f7256889c` | verbatim |
| `go/internal/query/github_login_handler.go` (version, alias, `GitHubLoginService`, request/response shapes, `Mount`) + `go/cmd/api/github_login.go` (adapter, 503) at `1e034786` | `contracts/upstream/github-login-wire-shapes.txt` | excerpt (line-presence at pin) | verbatim excerpt |
| `go/internal/query/saml_login_admission_contract_test.go` at `aac271ad` | `contracts/upstream/saml_login_admission_contract_test.go` | `bc04787bd208bf07428f4a9bc4660946342c7e4f3b60121202cb7f0a070556d0` | verbatim |
| `go/internal/query/saml_admission.go` (versions, alias) + `saml_handler.go` (`SAMLStore`, `SAMLProviderConfig`, `SAMLRequestCreateRecord`, `SAMLProviderIDLister`, `Mount`) at `aac271ad` | `contracts/upstream/saml-login-wire-shapes.txt` | excerpt (line-presence at pin) | verbatim excerpt |
| `go/internal/query/admin/recovery_admission_contract_test.go` at `7c605839` | `contracts/upstream/recovery_admission_contract_test.go` | `50dd78f3dc9bde8fdc67d7f818e0b2fdfdeb22fa7ce62d8650c361345826af24` | verbatim |
| `go/internal/query/admin/recovery_admission.go` (version, alias) + `handler.go` (`RecoveryService`, the four lane `Mount` lines) at `7c605839` | `contracts/upstream/recovery-wire-shapes.txt` | excerpt (line-presence at pin) | verbatim excerpt |
| `go/internal/query/admin/read_admission_contract_test.go` at `2c89a9d8` | `contracts/upstream/admin_read_admission_contract_test.go` | `51148149f97cd6613f6d7349d4f6dbb02ca55de9bf00e23cfcf410491cfe6784` | verbatim |
| `go/internal/query/admin/read_admission.go` (version, alias) + `handler.go` (`ReadStore`) at `2c89a9d8` | `contracts/upstream/admin-read-wire-shapes.txt` | excerpt (line-presence at pin) | verbatim excerpt |
| `go/internal/query/admin/identity/read_admission_contract_test.go` at `2c89a9d8` | `contracts/upstream/admin_identity_read_admission_contract_test.go` | `50e02d231061f28f261670bddeacec828e0e01f43269ab4cf9ed6045c217011f` | verbatim |
| `go/internal/query/admin/identity/governance_audit_read_admission_contract_test.go` at `2c89a9d8` | `contracts/upstream/governance_audit_read_admission_contract_test.go` | `b6b033cfdbdaf9cb788b1e7354e199caeda5b1604c7230e0b58caf97b3b2aef5` | verbatim |
| `go/internal/query/admin/identity/read_admission.go` (version, alias) + `governance_audit_read_admission.go` (version, alias) + `types.go` (`ReadStore`, `GovernanceAuditReader`) at `2c89a9d8` | `contracts/upstream/admin-identity-read-wire-shapes.txt` | excerpt (line-presence at pin) | verbatim excerpt |
| `go/internal/query/admin/acknowledgement/terminal_failure_read_admission_contract_test.go` at `2c89a9d8` | `contracts/upstream/terminal_failure_read_admission_contract_test.go` | `c0e86ef4e90ff24e6756f27bd870675782baeeeb9b1c37f9e0551e0cb035f51a` | verbatim |
| `go/internal/query/admin/acknowledgement/terminal_failure_read_admission.go` (version, alias, `TerminalFailureReader`) at `2c89a9d8` | `contracts/upstream/terminal-failure-read-wire-shapes.txt` | excerpt (line-presence at pin) | verbatim excerpt |
| `go/internal/query/openapi/paths/status/admin.go` (replay, reopen, refinalize, recover-generations entries only) at `7c605839` | `openapi/paths/status/admin_recovery.go` | `26c7669744ebb6b29a18bb9d95c7186e24785421afb8d733d9d9a3a246a78eb9` (whole file; this fragment is the line-verbatim subset) | verbatim subset |
| `go/internal/query/openapi/components.go` (`SkippedScopesReport`) + `components_recovery.go` (`DeltaActiveScopesReport`, `ReindexRequestsWrittenReport`) at `7c605839` | `openapi/components.go` (three lane schemas) | excerpt (line-presence at pin) | verbatim excerpt |
| `go/internal/query/openapi/paths/catalog/capabilities.go` at `17feac86e089` | `openapi/paths/catalog/capabilities.go` | `e29adb2cc78925212cc8e4c2882b063f2bcf15ff867b03a72a2febd2e8937229` | verbatim |
| `go/internal/query/openapi/paths/cli/score.go` at `17feac86e089` | `openapi/paths/cli/score.go` | `2cf44ef41567879b6ffc7f3cdc8468b51beaaf2e3ffcf528161c8fd65f954570` | verbatim |
| `go/internal/query/openapi/paths/cli/render.go` at `17feac86e089` | `openapi/paths/cli/render.go` | `6cdc25b8204c86417dd8659f2416561e2032a5792424659aa50516563e7bbba2` | verbatim |
| `go/internal/query/openapi/paths/cli/capture.go` at `17feac86e089` | `openapi/paths/cli/capture.go` | `9760b71696292fb8d83b79580d4e65bf9cc1dde0537e63e9f51c173f199b5aab` | verbatim |
| `go/internal/query/openapi/paths/cli/validate.go` at `17feac86e089` | `openapi/paths/cli/validate.go` | `159f8c031d1dbf8579a951a452648d3f7a52ec753ca538e8121b7ff4ec339b7f` | verbatim |
| `go/internal/query/openapi/paths/cli/compose.go` at `17feac86e089` | `openapi/paths/cli/compose.go` | `ef7a8b98c253956ee5bb8575e73d9af05373d3cf899e59221eed5feb370421c6` | verbatim |
| `go/internal/query/openapi/paths/search/ask.go` at `17feac86e089` | `openapi/paths/search/ask.go` | `6d1640ad03ddfa351a4df611d6faee451b3403dc39d02dee893d7115a8eb5740` | verbatim |
| `go/internal/query/openapi/paths/service/intelligence_report.go` at `17feac86e089` | `openapi/paths/service/intelligence_report.go` | `d7f43593264272966a11be43855a83595dc863545790e2f08711aa4a4472a747` | verbatim |
| `go/internal/query/openapi/paths/auth/routes.go` (AuthProviders entry only) at `17feac86e089` | `openapi/paths/auth/auth_providers.go` | `709fe6b3e96967fa615bff59a8c02cb8c1c5fbc6a05744bf57835170a5ad633a` (whole file; this fragment is the line-verbatim AuthProviders subset) | verbatim subset |
| `go/internal/query/openapi/paths/auth/admin_provider_configs.go` (AdminProviderConfigReads entry only) at `99381691b1ac` | `openapi/paths/auth/admin_provider_config_reads.go` | `afa45de81720f466b8b9f66949605564be21e9efa8ddb8c62e71600b127824ef` (whole file; this fragment is the line-verbatim AdminProviderConfigReads subset) | verbatim subset |
| `go/internal/query/openapi/paths/status/routes.go` (OpenAPISpec entry only) at `17feac86e089` | `openapi/paths/status/api_router_openapi.go` | `24a57b35c675cb33ad5011343a0aeb68d49e9d05d7645f397de2d88e8e5afa61` (whole file; this fragment is the line-verbatim OpenAPISpec subset) | verbatim subset |
| `go/internal/query/openapi/paths/status/admin.go` (the six query-POST entries only) at `7c605839` (admin.go is byte-identical at `7c605839` and `17feac86e089`) | `openapi/paths/status/admin_reads.go` | `26c7669744ebb6b29a18bb9d95c7186e24785421afb8d733d9d9a3a246a78eb9` (whole file; this fragment is the line-verbatim AdminReads subset) | verbatim subset |
| `go/internal/query/openapi/paths/auth/routes.go` (invitations GET entry only) at `17feac86e089` | `openapi/paths/auth/admin_invitations.go` | `709fe6b3e96967fa615bff59a8c02cb8c1c5fbc6a05744bf57835170a5ad633a` (whole file; this fragment is the line-verbatim get-only AdminInvitations subset) | verbatim subset |
| `go/internal/query/openapi/spec.go` (swaggerUIHTML, redocHTML shells only) at `17feac86e089` | `api/leafb_terminate.go` (leafbSwaggerUIShell, leafbReDocShell consts) | block-identical (whole-shell compare, see `check_docui_shell_at`) | verbatim subset |
| `go/internal/status/read_admission_contract_test.go` at `6b66e6b2` | `contracts/upstream/status_read_admission_contract_test.go` | `83f8a4a15c24aa93c3d82dc14da1880470be58301aeac10f4944f28f57a90233` | verbatim |
| `go/internal/query/supply/chain/supply_read_admission_contract_test.go` at `6b66e6b2` | `contracts/upstream/supply_read_admission_contract_test.go` | `8132db71af6058478332913bfd260190dfd98e8a09b9230a742b4d2ecc2a7367` | verbatim |
| `go/internal/query/admin/backfill_admission_contract_test.go` at `19de7346d1` | `contracts/upstream/backfill_admission_contract_test.go` | `50fb9462a3aa2d4869bf43cab7e8af69a92dc5a7a9625339fd6b86be34b5efbe` | verbatim |
| `go/internal/query/admin/dead_letter_admission_contract_test.go` at `19de7346d1` | `contracts/upstream/dead_letter_admission_contract_test.go` | `9c583e7ce8c06384b8c6642f1fea0c1d4581b40d84ca7dae3f94f2f192a56ddb` | verbatim |
| `go/internal/query/admin/reindex_admission_contract_test.go` at `19de7346d1` | `contracts/upstream/reindex_admission_contract_test.go` | `061092ea83a6447af59298e3df5d49ff7878eff0df72bc43e3b7c8fafef44767` | verbatim |
| `go/internal/query/admin/skip_admission_contract_test.go` at `19de7346d1` | `contracts/upstream/skip_admission_contract_test.go` | `0486124e708486faaa0dafd9e00fb387a03f228ea8676e9c82ca63c60d67eaae` | verbatim |
| `go/internal/query/openapi/paths/status/admin.go` (the four admin-four entries only) at `19de7346d1` | `openapi/paths/status/admin_d7.go` | `26c7669744ebb6b29a18bb9d95c7186e24785421afb8d733d9d9a3a246a78eb9` (whole file; this fragment is the line-verbatim AdminD7 subset) | verbatim subset |
| `go/internal/status/read_admission.go` (version, alias) + `go/internal/status/report.go` (`Reader`) at `6b66e6b2` | `contracts/upstream/status-read-wire-shapes.txt` | excerpt (line-presence at pin) | verbatim excerpt |
| `go/internal/query/supply/chain/supply_read_admission.go` (version, embedded ports) at `6b66e6b2` | `contracts/upstream/supply-chain-read-wire-shapes.txt` | excerpt (line-presence at pin) | verbatim excerpt |
| `go/internal/query/openapi/paths/status/collector_readiness.go` at `6b66e6b2` | `openapi/paths/status/collector_readiness.go` | `12f2e23ec324531f83c960eb9dc429c1da11f749cdce000e14b659d476da6f2f` | verbatim |
| `go/internal/query/openapi/paths/status/operator_control_plane.go` at `6b66e6b2` | `openapi/paths/status/operator_control_plane.go` | `fc58c514a42d02b56bd879f1b74678c3ee3c494bf8be49ef129525abda0f28c9` | verbatim |
| `go/internal/query/openapi/paths/status/semantic.go` at `6b66e6b2` | `openapi/paths/status/semantic.go` | `c50a5391e3d765cdf3d40f9999eae33ffa1e1a299df455f81903af63272578da` | verbatim |
| `go/internal/query/openapi/paths/freshness/causality.go` at `6b66e6b2` | `openapi/paths/freshness/causality.go` | `6409c05ee4b9c5078a7acabb12c3b783a297e2505a316f9b71d3bb2db5b0d3c2` | verbatim |
| `go/internal/query/openapi/paths/status/routes.go` (the seven lane entries only) at `6b66e6b2` | `openapi/paths/status/status_reads.go` | `24a57b35c675cb33ad5011343a0aeb68d49e9d05d7645f397de2d88e8e5afa61` (whole file; this fragment is the line-verbatim StatusReads subset) | verbatim subset |
| `go/internal/query/openapi/paths/supply/chain/runtime_context.go` at `6b66e6b2` | `openapi/paths/supply/chain/runtime_context.go` | `ec83120edab5d9b4d75622cc748d24c8f88f06036f8bc23e377a8b9830e6b83b` | verbatim |
| `go/internal/query/openapi/paths/supply/chain/advisory_catalog.go` at `6b66e6b2` | `openapi/paths/supply/chain/advisory_catalog.go` | `2316253e4c20862081a21778f2c6619bb4e1e35ba63de7c603a8b4069e122d3b` | verbatim |
| `go/internal/query/openapi/paths/supply/chain/advisory_evidence.go` at `6b66e6b2` | `openapi/paths/supply/chain/advisory_evidence.go` | `4a69d35575e23d8e8f7ed4453dbbd14c453688579e1f8edb320e35d74609d212` | verbatim |
| `go/internal/query/openapi/paths/supply/chain/container_images.go` at `6b66e6b2` | `openapi/paths/supply/chain/container_images.go` | `29ca221b684f1f3f1b88042121090f5f7ce136dae6812f8071e5dc911d1c42a1` | verbatim |
| `go/internal/query/openapi/paths/supply/chain/container_image_identity_aggregate.go` at `6b66e6b2` | `openapi/paths/supply/chain/container_image_identity_aggregate.go` | `457eaadc8b6c325b96bb399610577c2145fd7d77ba4e9902b8286f71b7b07600` | verbatim |
| `go/internal/query/openapi/paths/supply/chain/impact_explain.go` (payload only) at `6b66e6b2` | `openapi/paths/supply/chain/impact_explain.go` | `7b64a8ce69cc2802157303f1fd50a2914a5cd2c9693743368da0889639852f3e` (whole file; this fragment is the line-verbatim ImpactExplain subset) | verbatim subset |
| `go/internal/query/openapi/paths/supply/chain/impact_findings.go` (payload only) at `6b66e6b2` | `openapi/paths/supply/chain/impact_findings.go` | `0b0adcf656d2d14b8c470c3e46511d3e8d800a74dde92e68c4ffdde7278dce67` (whole file; this fragment is the line-verbatim ImpactFindings subset) | verbatim subset |
| `go/internal/query/openapi/paths/supply/chain/impact_aggregate.go` at `6b66e6b2` | `openapi/paths/supply/chain/impact_aggregate.go` | `9bcd0f907ab0012d86a79ee1a82339c16fd39a2535dec58d138cf625e61fec21` | verbatim |
| `go/internal/query/openapi/paths/supply/chain/sbom_attestations.go` at `6b66e6b2` | `openapi/paths/supply/chain/sbom_attestations.go` | `9571183b1fd617e39ecf214e5b9a679dcb73342b0b7f8c68c7a380f151fa710b` | verbatim |
| `go/internal/query/openapi/paths/supply/chain/sbom_attestation_attachment_aggregate.go` at `6b66e6b2` | `openapi/paths/supply/chain/sbom_attestation_attachment_aggregate.go` | `c8aad4c90a5a1f0973b542c172e9e492582ef82ae2112614a7170f34962cf856` | verbatim |
| `go/internal/query/openapi/paths/supply/chain/security_alerts.go` at `6b66e6b2` | `openapi/paths/supply/chain/security_alerts.go` | `3f8431bd296d43ab284ee97f3ba084bafcf2ae55f12d06ff3b7bf0473840b491` | verbatim |
| `go/internal/query/openapi/paths/supply/chain/security_alert_aggregate.go` at `6b66e6b2` | `openapi/paths/supply/chain/security_alert_aggregate.go` | `8a5709e57dd25f9263274828c4a470baa24236abf00a5de331a518a46dfd5088` | verbatim |
| `go/internal/query/openapi/paths/supply/chain/vulnerability_scanner_contract.go` at `6b66e6b2` | `openapi/paths/supply/chain/vulnerability_scanner_contract.go` | `3dbfbc45e39e80ba183ee7f0a45acd40cf1cd0de596fdd99dff63420326a6ca8` | verbatim |
| `go/internal/query/openapi/components_active_work_source.go` (`CoordinatorSource`, `TerraformStateSource`) at `6b66e6b2` | `openapi/components.go` (two D6 schemas) | excerpt (line-presence at pin, see `check_d6_components_at`) | verbatim excerpt |
| `go/internal/query/openapi/components.go` (ServiceName parameter) + `components_responses.go` (Unauthorized, NotImplemented) at `17feac86e089`, `components_provider_configs.go` (AdminProviderConfig, AdminProviderConfigWriteRequest) at `99381691b1ac` | `openapi/components.go` (D2 block) | excerpt (line-presence at pin, see `check_leafb_components_at`) | verbatim excerpt |
| `go/internal/query/openapi/paths/cicd/routes.go` at `17feac86e089` | `openapi/paths/cicd/routes.go` | `ddf2537b643884319ef98554f883a37a58f88bcab026ad94c32b34b747e07167` | verbatim |
| `go/internal/query/openapi/paths/cicd/run_correlation_aggregate.go` at `17feac86e089` | `openapi/paths/cicd/run_correlation_aggregate.go` | `f69d8339a3fa2a926ec5173f0655ac1bf1f3411b13438e310cff3f221bdb409d` | verbatim |
| `go/internal/query/openapi/paths/evidence/documentation.go` at `17feac86e089` | `openapi/paths/evidence/documentation.go` | `3fa8a202c0ac4a3935eb1b98c097c535e0ba95d17f0f07a2491815a458183635` | verbatim |
| `go/internal/query/openapi/paths/evidence/documentation_finding_aggregate.go` at `17feac86e089` | `openapi/paths/evidence/documentation_finding_aggregate.go` | `8c7eaa6fb39036ea12a65bd10ee092e534d620651c9b33a1fcbb25fae4db9b72` | verbatim |
| `go/internal/query/openapi/paths/evidence/bundle.go` at `17feac86e089` | `openapi/paths/evidence/bundle.go` | `f62e158efa2734993c1fb4c3ecd8d0a5f24e36343937bba3048fa29b9b4b48a2` | verbatim |
| `go/internal/query/openapi/paths/evidence/investigation_workflows.go` at `17feac86e089` | `openapi/paths/evidence/investigation_workflows.go` | `347f93d71ca8e54a7cb0350829261ffcb4661afdace73d69352912168f886054` | verbatim |
| `go/internal/query/openapi/paths/cloud/routes.go` at `17feac86e089` | `openapi/paths/cloud/routes.go` | `393f69d6bf1f71a5cd271c3e62fabb44ab22bf2a1ab3010243fe90ee30d478db` | verbatim |
| `go/internal/query/openapi/paths/infrastructure/routes.go` at `17feac86e089` | `openapi/paths/infrastructure/routes.go` | `6ce1fdca8609a6c4d6c56c7adf3ecfa6d31387ad64077c4780c62d245d03e136` | verbatim |
| `go/internal/query/openapi/paths/infrastructure/resource_aggregate.go` at `17feac86e089` | `openapi/paths/infrastructure/resource_aggregate.go` | `52caf0997f965428af702d0ebfbc99f3ada3007d3979e993f74c06ad01a64765` | verbatim |
| `go/internal/query/openapi/paths/repository/package_registry.go` at `17feac86e089` | `openapi/paths/repository/package_registry.go` | `b47f0506bdf450443d689bfeb815f4a72b4d2ba367e2686db6d5918c5472517c` | verbatim |
| `go/internal/query/openapi/paths/repository/package_registry_aggregate.go` at `17feac86e089` | `openapi/paths/repository/package_registry_aggregate.go` | `9cc3627c261b109d31e83e12b860c988671078d74affa88cf045e8429688d8eb` | verbatim |
| `go/internal/query/openapi/paths/repository/branches.go` at `17feac86e089` | `openapi/paths/repository/branches.go` | `1b92ee9e992dbf3cbca68983687ce31dde638193b16de965a3085d5222fd9d67` | verbatim |
| `go/internal/query/openapi/paths/repository/stats_and_coverage.go` at `17feac86e089` | `openapi/paths/repository/stats_and_coverage.go` | `2823f88f0659e77fa347ff72eb9f1bd3147343ad022efb737ab1201eb0bc293a` | verbatim |
| `go/internal/query/openapi/paths/repository/freshness.go` at `17feac86e089` | `openapi/paths/repository/freshness.go` | `9b60537e00fea4e8c6b535b9359498c9c0ee9d912fedca52562c6f0bdae78c4a` | verbatim |
| `go/internal/query/openapi/paths/catalog/playbooks.go` at `17feac86e089` | `openapi/paths/catalog/playbooks.go` | `f6a66c1873062dde7c6d7a3ef2bbc10e8293665f9e3971a61a3274f1909d2d46` | verbatim |
| `go/internal/query/openapi/paths/evidence/routes.go` at `17feac86e089` | `openapi/paths/evidence/evidence_routes.go` | `8c9950983d889af526fcd886ee02944f9179c99fde60c191c972c53605df563a` (whole file; this fragment is the line-verbatim EvidenceRoutes subset (drops the dangling core 501)) | verbatim subset |
| `go/internal/query/openapi/paths/evidence/investigations.go` at `17feac86e089` | `openapi/paths/evidence/deployable_unit.go` | `df0e51563c025a4c6571f31a04704c9abce4b9e33e613baadac5d4e316924cb0` (whole file; this fragment is the line-verbatim DeployableUnitPacket subset (deployable-unit entry only)) | verbatim subset |
| `go/internal/query/openapi/paths/repository/routes.go` at `17feac86e089` | `openapi/paths/repository/repository_routes.go` | `3641f90522c313023386476d92fce558219284e0cc76c886b6b6ecd07527ae65` (whole file; this fragment is the line-verbatim RepositoryRoutes subset (drops the mux-health and schema-interpolated story entries)) | verbatim subset |
| `go/internal/query/openapi/components.go` (RepoId, CollectorReadinessEnvelope, Repository, RepositoryRef, CatalogWorkload, InvestigationEvidencePacket, Relationship) + `components_active_work_source.go` (ActiveWorkSource) + `components_responses.go` (GatewayTimeout) at `17feac86e089` | `openapi/components.go` (D4 block) | excerpt (line-presence at pin, see `check_d4_components_at`) | verbatim excerpt |
| `go/cmd/api/CUTOVER_OWNERSHIP.md` | referenced only | `423109a914e4b9b457a7bdb0c626ecee273b7be0c18743c4396907021a24f66f` | census basis |
| `docs/internal/evidence/8872-eshu-api-destination-plan.md` | referenced only | `9dd775572bec0f86f390c0274dd03479fb1b79665c08117470ca3562ddf142b5` | content plan |

The upstream copies carry their original `SPDX-MIT` and copyright
headers unchanged. They are parity evidence, not build inputs: nothing in
this repository compiles them, and no core import path is redirected.

## Reproduce

Pin a source checkout and compare:

```bash
eshu=~/personal-repos/eshu
sha=e4c385f493494ed9d6c7f192b04ce0255361f621
git -C "$eshu" show "$sha:go/internal/query/querycontract/api_boundary.go" \
  | cmp - contracts/upstream/api_boundary.go
bash scripts/check-parity.sh
```

`check-parity.sh` performs the same comparison against freshly fetched
pinned sources, so CI does not depend on a local checkout.
