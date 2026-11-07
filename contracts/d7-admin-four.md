# D7 admin-four tier-C dispatch clones contract

Versioned pin record and clone-lane record for the four D7 operator
mutations (`POST /api/v0/admin/backfill`, `POST
/api/v0/admin/dead-letter`, `POST /api/v0/admin/reindex`, `POST
/api/v0/admin/skip`) (Refs `eshu-hq/eshu#9669`, part of
`eshu-hq/eshu#6707`). This lane registers the four explicit routes
inside `RequireAuth` ahead of the generic proxy with the AllScopes
family authorizer and forwards to the same core route with method,
path, query, headers, and body relayed; core bytes come back
unmodified and core re-judges every forwarded call as backstop. Dest
holds no backfill, dead-letter, reindex, or skip state.

## Pin records (satellite-side, D1/D6 precedent)

Pins minted core-side in `eshu-hq/eshu#10312` (merge
`19de7346d11a184bf88d8f307b5d7a9ced0a4657`), unwired seams, recorded
here byte-identical under `contracts/upstream/` at that commit:

| Core source at pin | Satellite record | sha256 | Treatment |
| --- | --- | --- | --- |
| `go/internal/query/admin/backfill_admission_contract_test.go` | `contracts/upstream/backfill_admission_contract_test.go` | `50fb9462a3aa2d4869bf43cab7e8af69a92dc5a7a9625339fd6b86be34b5efbe` | verbatim |
| `go/internal/query/admin/dead_letter_admission_contract_test.go` | `contracts/upstream/dead_letter_admission_contract_test.go` | `9c583e7ce8c06384b8c6642f1fea0c1d4581b40d84ca7dae3f94f2f192a56ddb` | verbatim |
| `go/internal/query/admin/reindex_admission_contract_test.go` | `contracts/upstream/reindex_admission_contract_test.go` | `061092ea83a6447af59298e3df5d49ff7878eff0df72bc43e3b7c8fafef44767` | verbatim |
| `go/internal/query/admin/skip_admission_contract_test.go` | `contracts/upstream/skip_admission_contract_test.go` | `0486124e708486faaa0dafd9e00fb387a03f228ea8676e9c82ca63c60d67eaae` | verbatim |

Each file pins `*AdmissionContractVersion = "v0"` plus the frozen
method set:

- `BackfillAdmission v0`: `RequestBackfill` only (call
  `facts.go:258`); `Store`-as-a-whole, `ReadStore`,
  `RecoveryService`, and `audit.Appender` explicitly out.
- `DeadLetterAdmission v0`: `DeadLetterWorkItems` only (call
  `facts.go:165`); the sibling `dead-letters/query` read stays under
  the D1 `AdminReadAdmission`; `Store`-as-a-whole, `ReadStore`,
  `RecoveryService`, and `audit.Appender` explicitly out.
- `ReindexAdmission v0`: embeds exactly the three branch ports —
  fleet `ReindexRequester`, repository `RepositoryCatalogMatcher`,
  repository `RepositoryReindexRequester` (branches at `reindex.go:58`,
  `reindex_repository.go:60`/:87); `Store`-as-a-whole, `ReadStore`,
  `RecoveryService`, `audit.Appender`, and the `reindex/progress`
  read explicitly out.
- `SkipAdmission v0`: `SkipRepositoryWorkItems` only (call
  `facts.go:212`); `Store`-as-a-whole, `ReadStore`,
  `RecoveryService`, and `audit.Appender` explicitly out.

`scripts/check-parity.sh` gates all four at `SRC_SHA_D7`
(`19de7346d11a184bf88d8f307b5d7a9ced0a4657`).

## Arbiter binding conditions (eshu-hq/eshu#10312, APPROVED-WITH-CONDITIONS)

The ruling's clone-lane conditions are binding on this lane, and each
is discharged or recorded below:

1. **Client-stable idempotency keys with request fingerprints.**
   Tier-C clones neither mint nor validate keys: the key bytes forward
   untouched and core judges them (`TestD7Native_IdempotencyKeyForwardsUntouched`:
   a present key arrives core-side byte-identical; a missing key still
   forwards exactly once and relays core's 400). The ledger that makes
   a retried key safe is the open termination obligation, not dest
   behavior. Dest invents, strips, or synthesizes no key field.
2. **Finalize/audit bound stack.** The real model is `WithoutCancel` +
   2s `defaultFinalizeTimeout` with a nested 500ms `audit.AppendTimeout`
   per append — and it runs entirely core-side on every forwarded
   request, because the handlers that own it still serve the request.
   Dest adds no finalize of its own; the only dest-side bound on the
   lane is the shared 30s dispatch timeout. Audit posture per clone:
   best-effort-after-commit core-side, unchanged from direct core
   (parity, not a behavior change) — dest appends nothing, and where
   core emits no allow-path event today (backfill, dead-letter,
   reindex, skip-allow) the clone emits none either. No fail-closed
   audit is introduced by this lane; the termination lane must state
   its choice explicitly.
3. **No single core object satisfies ReindexAdmission.** Dest composes
   nothing: this clone forwards to the already-composed core route
   (`POST /api/v0/admin/reindex`), which fans out to the fleet branch
   (default workspace scope: the fleet watermark, returned as
   `requested_at`) and the repository-scoped branch (each selector
   resolved to exactly one git default-branch scope; all-or-nothing —
   any unresolvable selector fails the request with 400 naming every
   such selector, and nothing is recorded). The per-backend fan-out
   the core handler waits for: the Postgres fleet watermark write, the
   repository catalog match reads, and the per-scope watermark writes,
   all behind the request checkpoint the 503 shapes name. Composing
   those ports dest-side belongs to the D0/transport lane, not here.
4. **Tier-C proof box.** Zero dest retries (single attempt in dest;
   transport 500s/503s retryable only end-to-end by the client, never
   by dest): every write-through case pins exactly one upstream hit,
   including the core-500, retryable-503, dead-core, pre-cancel, and
   mid-flight-cancel rows. Duplicate/no-worse-than-core proof:
   `TestD7Native_DuplicateIsNoWorseThanCore` delivers each route twice
   and pins 2 upstream hits with identical bodies and the pin's
   current-state effect count (backfill 2 — not even state-idempotent;
   dead-letter, reindex, and skip 1 — state-convergent). Scope-honesty
   label: tier-C dispatch only, not termination, not removal readiness
   (this document, `api/d7.go`, and the envelope below). Rollback:
   revert-to-relay (below).
5. **Serialization is not a substitute for the replay ledger.** Dest
   holds no queue, no dedup cache, no single-flight: the parallel
   probe pins 16 callers to 16 upstream hits. The ledger deficit stays
   open in this contract (see per-operation obligations).
6. **Termination stays last among mutations on D0-proven transport.**
   These are clones, not termination: the in-tree routes stay
   authoritative, and the served spec carries the clone label, not a
   termination claim.

## HTTP shape

- Backfill: explicit reason (and the key field the termination lane
  will claim; today core records the request without claiming it). A
  200 carries the recorded request; a re-run records a second row.
- Dead-letter: explicit work-item selection, reason, and the key field
  the termination lane will claim. A 200 sets the same `dead_letter`
  state again on re-run.
- Reindex: `ingester` (repository only), `scope` (workspace default,
  repository), `repositories` selectors (required with scope
  repository, rejected with scope workspace), `force` (must be true or
  omitted). A 202 carries the watermark(s); a re-run never lowers the
  watermark, and the repository-scoped path records nothing unless
  every selector resolves. Unknown body fields are rejected with 400.
- Skip: `repository_id` (a scope id or source key; a selector matching
  more than one scope fails closed with 409 naming the matched scopes)
  plus the operator note. A 200 counts only rows transitioned by the
  call; a re-run touches only rows still pending, retrying, or failed.
- Every mutating route gates on admin (all-scopes) authorization.
- Unconfigured handler (nil core origin): permanent 503
  `backend_unavailable`, never `Retry-After`.

## Per-operation idempotency/audit obligations (from the pin contracts)

Each pin self-gates its operation the same way: until the termination
lane lands the per-request key claimed core-side, the allow/deny audit
event via `audit.Appender`, and the bounded finalize, with fresh
arbiter review, the route stays relay-only. The clone lane does not
discharge these — it preserves them core-side by forwarding to the
handlers that own them:

- Backfill: claims no key, holds no ledger row, appends no audit
  event, runs no bounded finalize. Not even state-idempotent.
- Dead-letter: claims no key, holds no ledger row, appends no audit
  event, runs no bounded finalize. State-convergent; concurrent
  duplicates both execute.
- Reindex: claims no key, holds no ledger row, appends no audit
  event, runs no bounded finalize. Watermark-safe on re-run;
  concurrent duplicates both execute.
- Skip: claims no key, holds no ledger row, runs no bounded finalize.
  State-convergent; concurrent duplicates both execute. Only the deny
  path audits today (`skip_refused_ambiguous_scope`); there is no
  allow-path event.

## Native dispatch (tier C, D7)

`api/d7.go` registers the explicit routes inside `RequireAuth` ahead
of the generic proxy —

- `POST /api/v0/admin/backfill`,
- `POST /api/v0/admin/dead-letter`,
- `POST /api/v0/admin/reindex`,
- `POST /api/v0/admin/skip`

— each with the AllScopes family authorizer (plus the trailing-slash
twin on the same handler, so the family is never split across dispatch
paths). Method, path, query, headers (including the original
`Authorization` header), and body forward; core bytes come back
unmodified and core re-judges every forwarded call as backstop.

Scope honesty: dest holds the AllScopes authorizer, but with no
scoped/session resolver the deny branch is unreachable on the live
routes until one lands — the live 403s on these routes are core's.
The deny-shape test pins the contract for the resolver-landing day,
not a live path. Label the family tier-C dispatch (not termination,
not removal readiness) until the dest scoped resolver lands.

Cancel and timeout parity: dest never retries a forwarded mutation, so
a caller cancel or dispatch-timeout answer is the permanent local 503
`backend_unavailable` with exactly one upstream attempt
(`TestD7Native_CanceledContextFailsClosedWithoutRetry`, pinned on the
skip route; `TestD7Native_DeadCoreIs503WithoutRetry`, pinned on the
reindex route).

Audit parity: governance audit stays core-side on forwarded requests.
This lane re-implements nothing.
`scripts/check-parity.sh` gates the lane with the no-core-import proof
(`go list -deps` over the runtime packages must show no
`eshu-hq/eshu` import) alongside the carve-out, sealed-state, 401,
no-retry, envelope-parity, and ledger-deficit proofs above.

OpenAPI: the four route entries move as a line-verbatim subset of the
pinned core file at the D7 merge (satellite-design Decision 5(c); the
D7 merge leaves `admin.go` byte-identical to the recovery pin, same
sha256). The four path keys (`backfill`, `dead-letter`, `reindex`,
`skip`) are unique to this lane and separable from the C-c2, D3, and
relayed keys sharing that file: none of the four entries references a
shared schema (only the versioned response `$ref`s every lane already
carries), so this lane ports no schemas alongside the entries.
Assembly adds no new components; `TestSpec_LocalRefsResolve` fails
closed on a dangling `$ref`. The served dest spec therefore covers
exactly the landed families, and no served key duplicates another
lane's route.

## Envelope scope-honesty label

Client-visible scope posture for the four routes: `tier-C dispatch`
— the destination forwards the request to the authoritative in-tree
route and relays its answer; it does not terminate the route
dest-side, resolve scoped/session identity, mint idempotency keys,
keep a ledger, append audit, or bound finalize. Termination and removal
readiness are explicitly not claimed.

## Rollback

Additive. Revert this lane's dest commit (or re-pin the prior digest)
and the four admin-four routes fall back to the generic byte-relay —
where they answer dest 401s until the lane returns. No core change, no
migration, no flag; no dest-side backfill, dead-letter, reindex, or
skip state exists to unwind. Verify by re-running the lane
conformance suite against the reverted tree: the ahead-of-relay
assertions go red (the routes relay again) while every other suite
stays green. No Plan A rehearsal is consumed by this rollback — it
restores a proxy path, not durable state.
