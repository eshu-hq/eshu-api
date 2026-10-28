# Parity and compatibility

Seed for `eshu-hq/eshu#8872` (part of `eshu-hq/eshu#6707`). This note
records what moves, what stays, the compatibility rule, and the proofs
still open. It is a preparation record, not a cutover claim.

## Census basis

`go/cmd/api/CUTOVER_OWNERSHIP.md` at the provenance pin maps all 270
live API routes: 220 `read` rows backed by reader ports only
(`db.ReadStore`, graph `GraphQuery`, `ContentStore`), 50 `mutation` rows
persisting through core-owned admission storage, mounted by `newRouter`
(244) or `wireAPI` (26). The OpenAPI fragments under
`go/internal/query/openapi/` publish the request/response shapes; the
spec assembly order is the compatibility surface the destination owns
after the move.

## What moves

- Read families (all 220 `read` rows): consumers of the versioned core
  read contracts recorded in `contracts/core-read-contracts.md`. No
  durable writes travel with them.
- Mutation HTTP shapes (all 50 `mutation` rows): the routes move, the
  stores do not. Each persists through an explicit versioned core
  admission contract; the only in-tree seam is the suppression admission
  recorded in `contracts/suppression-admission.md`.
- OpenAPI contract fragments and the `api/` server, router, auth, v1
  alias, and deprecation middleware: move in the follow-up extraction
  PR, reviewed row-for-row against the ownership map. This seed carries
  only the versioned seams, not the fragments.

## What stays in the core

Fact admission, canonical identity, projection, retraction, recovery,
graph truth, queue and lease state, schema bootstrap, and one-shot
repair. `wireAPI`-mounted families keep their keyring and resolver
composition documented as required construction inputs, not hidden
globals.

## Compatibility policy

`eshu-api` supports the exact pinned core contract versions in
`docs/provenance.md` (`CoreReadContractsVersion v0`,
`SuppressionAdmissionContractVersion v0`). A breaking change to any
named port fails the frozen-set test next to that port in the source
(`TestCoreReadContractsVersioned`,
`TestSuppressionAdmissionContractIsVersioned`,
`TestReadStoreContractIsVersioned`) until the version bumps in the same
diff; this repository then records the new pin before consuming it. No
shared-table shortcut is ever a compatible change.

## Open proofs (NOT_CHECKED)

Representative and hostile fixtures, auth and authz, pagination,
truth and error envelopes, and OpenAPI agreement are proven per family
by the in-tree suite cited in review. Still required before any
cutover: extracted-path parity on QA EKS, concurrent
retry and duplicate-delivery behavior, ordering, cancellation, revoked
authority, stale claims, partial and ambiguous failures, resource
bounds, no-regression measurements, health and readiness signals,
credential handling, artifact provenance, supported version pairs,
independent build and release, rollback rehearsal, and independent
review. The in-tree implementation is removed after production
acceptance, not before.
