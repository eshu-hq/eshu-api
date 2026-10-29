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
| `api/` | HTTP skeleton: router, auth gate, v1 alias, v0 deprecation |
| `internal/readclient/` | Versioned HTTP read-contract client (no core imports) |
| `cmd/eshu-api/` | Binary: health/readyz today, core dispatch next |
| `contracts/` | Versioned contract surface and byte-exact upstream copies |
| `contracts/upstream/` | Verbatim source copies; parity-checked, not build inputs (nested module marker keeps the toolchain out) |
| `docs/provenance.md` | Source pins, file mapping, reproduction commands |
| `docs/parity.md` | What moves, what stays, compatibility policy, open proofs |
| `docs/skeleton.md` | What this increment serves, credential posture, NOT_CHECKED |
| `scripts/check-parity.sh` | Parity gate: re-fetches pinned sources, compares bytes |
| `Dockerfile` | Multi-stage pre-cutover image (PRs prove it via the `image` lane) |
| `.github/workflows/release.yml` | Tag `v*` release: GHCR push with provenance, SBOM, Trivy gate (see `docs/release.md`) |

## Status

Skeleton only. The route census this work builds on is
`go/cmd/api/CUTOVER_OWNERSHIP.md` in `eshu` (271 rows: 221 reads backed
by reader ports, 50 mutations through core-owned admission stores,
measured from the current map; the seed parity basis recorded 270/220 at
the pinned commit — see `docs/provenance.md`). This
increment serves health, readiness, and the auth-gated `/api/*` chain
(401 envelope, v1 rewrite, v0 deprecation headers, explicit 503 until
core dispatch lands); no core route is served and nothing here carries
production authority. See `docs/skeleton.md` for this increment and
`docs/parity.md` for the proofs still required before any cutover,
including extracted-path parity on QA EKS, independent review, and a
rehearsed rollback.
