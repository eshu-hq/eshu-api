# Source provenance

Seed for `eshu-hq/eshu#8872` (part of `eshu-hq/eshu#6707`). Status:
preparation only; nothing here carries cutover authority.

## Pins

| Artifact | Pin |
| --- | --- |
| Source repository | `eshu-hq/eshu` |
| Source commit | `e4c385f493494ed9d6c7f192b04ce0255361f621` (`origin/main` at seed time) |
| Destination base | `a49341a8eb73ee620e97df4b88b5ddf93efc62c5` (README-only `main`) |
| `CoreReadContractsVersion` | `v0` |
| `SuppressionAdmissionContractVersion` | `v0` |

## Mapping

`contracts/upstream/` files are byte-exact copies of the pinned sources;
`scripts/check-parity.sh` re-fetches each source at the pinned commit and
compares bytes. The census and destination plan are referenced, not
copied: they keep evolving in the source until the extraction PR.

| Source path at pin | Destination | sha256 at pin | Treatment |
| --- | --- | --- | --- |
| `go/internal/query/querycontract/api_boundary.go` | `contracts/upstream/api_boundary.go` | `c5f3ab34bc6e8d43c119b9aa7028ba298fb7f15cc68ffd8386e63ad9c140d9a1` | verbatim |
| `go/internal/query/querycontract/api_boundary_test.go` | `contracts/upstream/api_boundary_test.go` | `346fd5fe3a40257812108e7d4fb72c3e683a28f0412e121781e8bcb9349a92b8` | verbatim |
| `go/internal/query/supply/chain/suppression_admission_contract_test.go` | `contracts/upstream/suppression_admission_contract_test.go` | `f9e08e5ee8f69f6ffb97a14be6995c7f9497e67f8317c5338166a83246318552` | verbatim |
| `go/internal/storage/postgres/db/read_store_contract_test.go` | `contracts/upstream/read_store_contract_test.go` | `b4d021da1c406870555ab4d414f1e7962ddf187d3c996e8012f10ddd70cd3166` | verbatim |
| `go/internal/query/supply/chain/suppression_mutation.go` lines 44-82 | `contracts/upstream/suppression-wire-shapes.txt` | excerpt `49b366852ef87eb5420969fd60cf99e8538c85b639d361c64506c31cffa930ac` | verbatim excerpt |
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
