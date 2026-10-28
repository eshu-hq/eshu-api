# Core read contracts

Versioned read-contract set the API surface consumes (Refs
`eshu-hq/eshu#8872`). Read runtimes display core truth; they never take
direct table or queue handles.

## Version

`querycontract.CoreReadContractsVersion = "v0"`, declared in
`contracts/upstream/api_boundary.go` (verbatim source copy).

## Ports

| Port | Source declaration | Frozen method set |
| --- | --- | --- |
| `GraphQuery` | `go/internal/query/querycontract/ports.go` | `frozenCoreReadPorts["GraphQuery"]` in `contracts/upstream/api_boundary_test.go` (2 methods: `Run`, `RunSingle`) |
| `ContentStore` | `go/internal/query/querycontract/ports.go` | `frozenCoreReadPorts["ContentStore"]` in `contracts/upstream/api_boundary_test.go` (35 methods) |
| `ReadStore` | `go/internal/storage/postgres/db` | `frozenReadStore` in `contracts/upstream/read_store_contract_test.go` (3 methods: `BeginReadOnlySnapshot`, `QueryContext`, `QueryRowContext`) |

The full method sets live in the verbatim test copies above; they are
not repeated here so this record cannot drift from the checked copies.
`scripts/check-parity.sh` proves those copies are byte-exact against the
pinned source commit.

## Enforcement

`TestCoreReadContractsVersioned` (in the vendored querycontract test)
pins `GraphQuery` and `ContentStore`; `TestReadStoreContractIsVersioned`
(in the vendored db test) pins `ReadStore` and asserts the same shared
version. A renamed, added, removed, or re-signed method fails its test
until `CoreReadContractsVersion` bumps in the same diff. Consuming a new
version requires recording the new pin in `docs/provenance.md` first.
