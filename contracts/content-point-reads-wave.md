# Wave-2 Content point reads termination contract

Versioned read-termination record for the Wave-2 Content wave (Refs
`eshu-hq/eshu#9669`, D13 tracker `eshu-hq/eshu#10491`, satellite
`eshu-hq/eshu-api#72`). This lane terminates exactly the 3 Content
point-read POSTs natively against the D0-proven terminated transport:
dest owns the route (caller gate, family authorizer, response mapping)
and calls the versioned core read API as its port transport. Dest holds
no table or queue handle; authz stays atomic core-side.

Scope source: the ContentPointReadAdmission v0 pin
(`contracts/content-point-read-admission.md` at core `c987f73`, merge of
`eshu-hq/eshu#10504`): the bounded single-file content lookup, the
bounded single-file line-range slice (clamping and empty-range edges
included), and the bounded single-entity lookup by entity id behind
the content point routes.

Transport pins:

- `ContentPointReadAdmission v0`
  (`contracts/upstream/content_point_read_admission_contract_test.go`
  at the Wave-2 merge SHA, plus the
  `contracts/upstream/content-point-read-wire-shapes.txt` excerpt)
  covers all 3 wave rows — every row admits through its re-declared
  port method: `POST /api/v0/content/files/read` through
  `GetFileContent`, `POST /api/v0/content/files/lines` through
  `GetFileLines`, `POST /api/v0/content/entities/read` through
  `GetEntityContent`.

The minting rule forbids inventing pins beyond it: the list legs
(`ListRepoFiles`, `ListRepoEntities`, twins) and the scoped-entity
leg (`GetEntityContentInRepositories`) stay explicitly out per the pin
record and stay relay with the reason disclosed per route below. The
search legs (`SearchFileContent`, `SearchEntityContent`, twins) were
out of this pin and relayed until the W4 lane terminated them under
their own pin (`ContentSearchReadAdmission v0`,
`contracts/content-search-termination.md`,
satellite eshu-hq/eshu-api#111) — they are not claimed here.

## Content

What this lane proves per the D-wave ruling is the required proof
list: explicit routes ahead of relay, byte-identical fragments under
`scripts/check-parity.sh`, authorizer + deny-shape tests with the
scope-honesty label, byte-parity dispatch/conformance tests vs the
relayed reference (including the line-range slicing-edge bodies the
pin shape requires), the no-core-import check, extended
`viaNative`/`viaRelay` telemetry, the satellite latency evidence
(terminated-vs-proxy-relay ratios, no same-shape before/after: the
baseline was the file-search proxy route until the W4 lane terminated
it, then the still-relayed code bundles POST —
`docs/content-latency.md`), and this
contract section with rollback.

## Production binding

- Transport: the existing versioned core HTTP read API
  (`CoreReadContractsVersion v0`), bounded client (30s, no redirect,
  no retry; no dest-side byte budget — `forwardToCore` streams the
  body unbounded), forwarded credential and negotiation headers,
  guaranteed correlation ID, fail-closed local 503 (permanent, never
  `Retry-After`) when no core origin is configured.
- Envelope negotiation (eshu-hq/eshu-api#66): success shape stays
  core-owned. The edge forwards the caller `Accept` verbatim and
  relays the core bytes verbatim — it never wraps a flat core answer
  into `{data, truth}`. Dest-synthesized errors stay envelope-aware
  via `writeError`.
- Trust model: dest's only constructible identity is the shared key,
  which core maps to AllScopes. The dest authorizer allows it; core
  re-judges every forwarded call as backstop (tenant scope, permission
  catalog, governance mode) — which is also where the live 403s and
  mode-dependent denials come from. Dest adds no handler validation and
  emits no audit: request/response semantics stay core-owned. Scoped
  tokens receive only granted repositories per the pin fragments; an
  ungranted selector is core's not-found. The dest gate relays those
  classes explicitly so core judges them.
- Reads emit no governance audit events on either path: the in-tree
  content handlers gate on the repository grant but never append, so
  there is no audit write to keep atomic; idempotency is the POST
  repeat check in the conformance matrix.

## Per-family record

Scope-honesty label (content family): the dest authorizer covers the
shared-key (AllScopes) identity only. Scoped tokens, OIDC bearers, and
browser sessions relay explicitly — core judges, core bytes come back
untouched, dest invents no 401/403/404/500 for them — until the D8
session resolver lands. The deny branch (`allScopes=false`) is pinned
for that day and unreachable on live routes.

| Family | Routes | Serve | Authorizer note |
| --- | --- | --- | --- |
| Content | 3 POSTs (`content/files/read`, `content/files/lines`, `content/entities/read`) | wave-gated terminate / relay-explicit | shared-key allow |

Cookie rule (D2 follow-up lesson, shared with the D3/D4/D6 and Wave-1
gates): only session-cookie-shaped credentials relay — a Cookie member
named `eshu_session` (or `__Host-eshu_session`, the two names the core
caller-credential reader accepts) relays explicitly; any other Cookie
content is junk, not a browser session, and stays dest-401 with zero
core contact.

Known divergences (disclosed, not hidden):

- No wave route stays relay: all 3 pinned POSTs terminate. The
  remaining out-of-pin legs stay out with no wire change: the list
  legs and the scoped-entity leg were never served here and keep
  their own standing until a deliberate pin covers them. (The search
  legs left the relay with the W4 lane —
  `contracts/content-search-termination.md`.)
- Non-POST spellings of the 3 paths are not registered: a keyless GET
  keeps the standing 401 at the key gate; a keyed GET crosses the
  generic relay and core judges the method (core allowlists these
  routes POST-only).
- Unconfigured origin answers the permanent local 503 on terminated
  routes where core nil-safe handlers would answer zero-defaults —
  the standing fail-closed rule, uniform across native lanes.

## Spec posture

The Wave-2 fragment is a line-verbatim subset port of the pinned core
content fragment: the 3 point-read entries land in
`openapi/paths/search/content_point_reads.go` (const
`ContentPointReads`) and assemble right after the ask entry. The core
file carries the lane entries mid-list with separator commas — not a
paths-closing entry shaped like the Compare fragment — so the subset
concatenates without re-shaping and no disclosed absence is recorded.
The core file's search entries shipped with the W4 lane in
`openapi/paths/search/content_search.go`
(`contracts/content-search-termination.md`) — no served key duplicates
a relayed route. The lane adds
the two schemas its entries reference (`FileContent`,
`EntityContent`), ported line-verbatim into `openapi/components.json`
from the pinned core components source. No fragment carries a
`\"security\": []` public-route marker: all 3 are authed reader-port
reads, and the posture test (`TestSetupSpecPosture`) is unaffected —
the only served marker stays the permanently-public login-picker entry
(`GET /api/v0/auth/providers`).

## Rollback

Single-commit revert: remove the `s.registerContentPointReads(mux)`
call in `api/router.go` (with `api/content_wave.go`,
`api/content_terminate.go`, and their tests) and the lane reverts to
the generic byte-relay with zero state change (core untouched, no
durable dest state, no data migration). The served spec drops the 3
keys with the fragment; the relay legs keep serving them from core.
Verify by re-running the lane conformance suite against the reverted
tree: the ahead-of-relay assertions go red (routes relay again) while
every other suite stays green. No Plan A rehearsal is consumed by this
rollback — it restores a proxy path, not durable state.
