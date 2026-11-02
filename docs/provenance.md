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
