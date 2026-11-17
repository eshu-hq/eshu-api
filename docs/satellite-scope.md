# Satellite scope statement (eshu-hq/eshu-api#69)

Explicit scope-shrink record for the Path B scoped-inventory redesign
(part of eshu-hq/eshu#9669; Row-7 ruling: Path A vs Path B is an owner
decision, owner acceptance still owed).

## What the satellite serves

The satellite serves its own mounted scope only — never the full core
surface. Current pin (D13 row-7 full-surface lane, satellite
`eshu-hq/eshu-api#153`, pinned by `TestSpecScope_Frozen`):

- Mounted: 284 explicitly registered operations (284 native + 0
  explicit relay holds — the retired D4 story relay leg now serves
  natively), rebuilt from the router tables by
  `TestSpecScope_Frozen`.
- Disclosed absences: 5 by-design non-API exclusions with no served
  spec entry (the three mux-level non-API mounts and the two compiled
  doc-UI shells, each backed by its rationale in `specScopeAbsent`);
  the disclosed API-absence set is zero — the row-7 lane closed every
  disclosed posture as served.
- Served: 279 operations at `GET /api/v0/openapi.json`, pinned
  operation-for-operation in the committed snapshot
  `openapi/spec_snapshot.json` (spec v0, document sha256 pinned
  alongside the sorted operation list).

## What the satellite does not serve

The row-7 lane closed the disclosed-absence set to zero: the served
document covers the full CUTOVER route surface (279 served = 284
mounted minus the 5 by-design non-API exclusions). What stays out of
the document is the unbounded generic `/api/` byte-relay catch-all —
catch-all riders are not bounded registrations and never gain spec
entries — plus the 5 non-API mounts, which are not API operations. A
byte-identical satellite-for-core spec pin (Path A) stays out of scope
for core surface outside the mounted scope, which still relays; this
statement scopes the satellite to the snapshot instead.

## Drift gate

Two gates consume the committed snapshot file and fail closed on
drift — owning lanes re-pin the snapshot with their change:

- `openapi/spec_snapshot_test.go`: the assembled `Spec()` document
  (operations and full-document sha256) equals the snapshot.
- `api/spec_snapshot_test.go`: the document served through the real
  route, for both a configured and an unconfigured core origin,
  equals the snapshot and reconciles with mounted-minus-absent.

Both gates read the committed file plus in-process serving only.
Never point a drift gate at a live satellite fetch: that measures a
deployment, not the committed tree.

## Spec-endpoint auth posture (decided and pinned)

The spec endpoint stays inside `RequireAuth` like every other dest
`/api/*` route: unauthenticated callers get 401, authenticated
callers get the document. Core serves its spec publicly; dest adds no
new public surface here. Pinned by
`TestSetupSpecPosture_StaysCoreSide` and documented at the route
registration in `api/router.go`. This is the all-authed branch of the
acceptance criterion, honestly documented — no 401-vs-public
mismatch is claimed.

## Tier-C dispatch exclusion

Tier-C dispatch families (D7 admin-four, D8 session-resolver, D9
login lanes, setup mutations, and the generic `/api/` relay itself)
are dispatch only — not termination — and carry zero removal credit.
Recorded where each lane lives (`api/d7.go`, `api/d8.go`, the D8/D9
conformance tests, `contracts/d9-saml-login.md`), restated here so
no removal-credit claim can attach to them: termination proof, with
byte-parity, is each family lane's own exit gate.

## Re-pinning

Regenerate the snapshot from `Spec()` in-process (never from a live
fetch), keep the operation list sorted, and land it with the lane
that moves the served set. The gates fail red until the snapshot
moves with its lane.
