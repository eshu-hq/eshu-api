# eshu-api

Reads, search results, and admin-policy HTTP shapes for Eshu, served from
versioned core contracts. Pre-release: this repository holds no cutover
authority until its reviewed cutover lands. Refs `eshu-hq/eshu#8872`, part
of `eshu-hq/eshu#6707`.

## Boundary

`eshu-api` owns the HTTP surface: routing, auth middleware, the v1 alias
and deprecation behavior, and the request/response shapes documented under
`contracts/`. It never takes direct table or queue handles.

The core keeps everything durable: fact admission, canonical identity,
projection, retraction, recovery, graph truth, queue and lease state,
schema bootstrap, and one-shot repair (`go/internal/storage/postgres`,
reducer, graph writers, admission stores in `eshu`). Sharing a Postgres
cluster during migration does not promote its tables to a public API.

Reads consume the versioned core read-contract set
(`querycontract.CoreReadContractsVersion`, currently `v0`): the
`GraphQuery` and `ContentStore` ports plus the Postgres `ReadStore`
surface. Mutations keep their HTTP shape here and persist through explicit
versioned core admission contracts, one per mutation family; the only
in-tree seam so far is the bounded suppression admission
(`query.SuppressionAdmission` at `SuppressionAdmissionContractVersion`,
currently `v0`).

## Layout

| Path | What it holds |
| --- | --- |
| `contracts/` | Versioned contract surface and byte-exact upstream copies |
| `contracts/upstream/` | Verbatim source copies; parity-checked, not build inputs |
| `docs/provenance.md` | Source pins, file mapping, reproduction commands |
| `docs/parity.md` | What moves, what stays, compatibility policy, open proofs |
| `scripts/check-parity.sh` | Parity gate: re-fetches pinned sources, compares bytes |

## Status

Preparation only. The route census this seed builds on is
`go/cmd/api/CUTOVER_OWNERSHIP.md` in `eshu` (270 rows: 220 reads backed
by reader ports, 50 mutations through core-owned admission stores). No
route implementation moves in this seed, and nothing here is served from
production. See `docs/parity.md` for the proofs still required before any
cutover, including extracted-path parity on QA EKS, independent review,
and a rehearsed rollback.
