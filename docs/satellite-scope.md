# Satellite scope statement (eshu-hq/eshu-api#69)

Explicit scope-shrink record for the Path B scoped-inventory redesign
(part of eshu-hq/eshu#9669; Row-7 ruling: Path A vs Path B is an owner
decision, owner acceptance still owed).

## What the satellite serves

The satellite serves its own mounted scope only — never the full core
surface. Current pin (re-verified at origin/main `64ce419`):

- Mounted: 219 explicitly registered operations (218 native + 1
  explicit D4 story relay leg), rebuilt from the router tables by
  `TestSpecScope_Frozen`.
- Disclosed absences: 45 mounted spellings with no served spec entry,
  each backed by its owning contract (`specScopeAbsent`).
- Served: 174 operations at `GET /api/v0/openapi.json`, pinned
  operation-for-operation in the committed snapshot
  `openapi/spec_snapshot.json` (spec v0, document sha256 pinned
  alongside the sorted operation list).

## What the satellite does not serve

The in-tree core inventory recomputed at eshu-core `820f9b6` counts
278 operations over 269 `/api` path keys (method+path entries in
`go/internal/query/openapi/paths`), against 174 served here. The
delta — roughly 104 operations — rides the keyed generic `/api/`
byte-relay (or the explicitly mounted relay legs) with no spec entry
and no removal credit. A byte-identical satellite-for-core spec pin
(Path A) stays impossible until family lanes port those fragments;
this statement scopes the satellite to the snapshot instead.

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
