# OIDC-pair native termination

Second of the sequential termination waves (Refs `eshu-hq/eshu#9669`,
satellite `eshu-hq/eshu-api#72`, arbiter APPROVED-WITH-CONDITIONS, D9
order OIDC after the signinpolicy-group merge, one family per PR).
This lane terminates the relay legs for exactly two routes so the
satellite natively terminates them:

- `GET /api/v0/auth/oidc/login` (public login start)
- `GET /api/v0/auth/oidc/callback` (public login callback)

GitHub and SAML stay on their landed tier-C write-through lanes;
this lane never names them. Core repo takes no code in this wave.

## What termination means here

Both routes register as exact-path native-public handlers outside
every gate, ahead of the generic proxy (see `api/router.go`). Dest
owns the route mapping over the versioned `OIDCLoginAdmission` v0
port (`oidcLoginPort.StartOIDCLogin` + `CompleteOIDCLogin`, the D0
proof): each terminated GET makes exactly one anonymous core fetch
over D0-proven transport with the caller query crossing as opaque
bytes, and dest synthesizes the answer from the port result — no
core bytes cross the generic relay on these two paths.

- Login start: the 302 answer carries the provider authorization
  `Location` byte-verbatim. Failures fail closed without leaking
  provider or claim details: the disabled-provider 503, the 400
  invalid, the 401, and the 403 denied shapes relay core bytes
  verbatim; only the unreachable/unconfigured core answers the
  dest-synthesized permanent local 503 (`backend_unavailable`,
  never `Retry-After`).
- Callback: the 201 answer carries the browser-session JSON plus the
  two `Set-Cookie` session cookies byte-verbatim
  (name/value/attrs/expiry); the 303 answer carries both cookies
  before redirecting to the stored same-origin return path, with its
  `Location` byte-verbatim. Denied logins (403, unmapped groups or
  revoked grants) create no session because core created none.
- Exactly-once stays core-side (`ConsumeState`): a parallel
  duplicate callback makes one fetch per delivery — dest holds no
  lock and serializes nothing — the first wins one session and every
  loser relays core's error.

Dest takes no table or store handle anywhere in the lane
(`api/oidc_terminate.go` references no `sql`, store, table, or DB
handle — only prose mentions of core-side stores), mints no session,
keeps no replay cache, appends no audit event, and runs no finalize.
Governance audit stays core-side: the identity-authentication event
emits from inside the core callback handler that still serves every
fetch.

## What stays

- `GET /api/v0/auth/github/login` + `GET /api/v0/auth/github/callback`
  stay on the GitHub write-through lane; the SAML triple stays on the
  SAML lane. Neither is named outside the boundary tests that pin
  them untouched.
- Trailing-slash twins, deeper subtrees, and non-GET spellings of the
  two OIDC paths stay on the generic relay behind RequireAuth:
  keyless callers keep the standing 401 there with zero core contact;
  keyed callers get core's answer for the spelled path.
- Docs/redoc shells, the leafb/D3/D4/D6 terminated waves, the D7/D8
  dispatch lanes, the setup carve-out, and the signinpolicy-group
  termination are untouched (parallel-safe: the router hunks sit
  ~170 lines apart with no shared lines; this lane lands only after
  the group-1 merge).

## Group gates (evidence)

- D0 proof: `TestOIDCTermPortIsFrozen` pins the dest-side frozen
  port (two methods, exact frozen signatures); production calls it on
  both terminated legs; `TestOIDCTerm_LaneFileHygiene` greps no
  store/table/lock primitive in the lane file.
- Pin byte-identical: `OIDCLoginAdmission` v0 at core SHA
  `40e88be128ca876f7333588137af5a780bf31b93` (see
  `contracts/d9-oidc-login.md` + `SRC_SHA_D9` in
  `scripts/check-parity.sh`, unchanged by this lane); D8 session
  pins complete at the re-anchored SHA (BrowserSession shape carries
  the callback cookies — this lane mints none itself).
- Public-registration exactness: `TestOIDCTermGroup_PublicExactnessRED`
  (10 non-carved spellings keyless 401, zero hits),
  `TestOIDCTermGroup_KeyedExactIsNative` (keyed exact paths file
  `via="native"`), `TestOIDCTermGroup_KeyedTwinRelays` (keyed twins
  relay core bytes for the spelled path, never synthesized),
  `TestOIDCTermGroup_V1AliasTerminates` (both v1 spellings
  terminate).
- Byte parity terminated-vs-relay for the full D9 outcome matrix
  (start 302 incl. duplicate, callback 201/303 with cookies,
  unconfigured 503, validation 400, callback 401, denied 403, core
  500, core 503 retryable/permanent) in both negotiated shapes:
  `TestOIDCTerm_MatchesRelayByteForByte` (status, body, Location,
  Retry-After, Content-Type, Set-Cookie per key in order).
- Redirect proof: `TestOIDCTerm_RedirectUnfollowed` (302 relays its
  Location, exactly one upstream hit),
  `TestOIDCTerminated_LoginSynthesizesRedirect` /
  `TestOIDCTerminated_CallbackSynthesizesSession` (redirect +
  session cookies byte-verbatim, one fetch each).
- Error + disabled-provider deny shape: `TestOIDCTerm_NoRetryOnUnstable`
  (500/retryable-503/permanent-503 verbatim, one attempt each),
  `TestOIDCTerm_DisabledProviderDenyShape` (unavailable 503
  verbatim), `TestOIDCTerm_DeadCoreSingleAttempt` + `TestOIDCTerm_UnconfiguredOriginFailsClosed`
  (permanent local 503, never `Retry-After`).
- Exactly-once replay under parallel duplicate callback: one session,
  loser core error — `TestOIDCTerm_ParallelDuplicateCallbackExactlyOnce`
  (16 parallel deliveries, one 201, fifteen 403s, 16 fetches, never
  serialized — proved, plus the no-`sync`/`Mutex` grep).
- Opaque-bytes callback: `TestOIDCTermNative_CredentialHygiene`
  (state/code/Location/cookie canaries absent from logs and
  metrics), `TestOIDCTerm_LaneFileHygiene` (no `.Query(`,
  `ParseForm(`, `FormValue(`, `.RequestURI`, credential literals,
  logger/tracer, store, or lock in the lane file),
  `TestOIDCTerm_KeylessCarriesNoCredential` (no shared-key
  injection; caller keys cross untouched per the D9 pin).
- OpenAPI fragments line-verbatim: unchanged fragments, parity
  green at the pinned SHA (see above);
  `TestOIDCTermGroup_SpecCarriesBothOps` pins the assembly.
- Telemetry: `TestOIDCTerm_TerminatedFilesNoAuthorizer`
  (native `core_ok` per leg, port fetches timed, no authorizer
  rows — public routes file no judgment).
- 5-leg latency: `scripts/measure-oidc-termination-latency.sh`
  (login bare, login full-query, callback vs the GET relay leg;
  login-query and callback vs the POST relay leg; 1.50 pathology
  bound). Parity proof runs in isolation AND under the full
  parallel race+coverage load (see proof commands): the SAML-lane
  native-307-vs-relay-503 flake under full parallel load is the
  caution — an isolation-only green is not done.
- Fail-closed box: unconfigured origin, dead core single attempt,
  unreadable/oversize body, unbuildable fetch — all permanent local
  503, never `Retry-After`.

## Rollback

Revert-to-relay: re-point the two exact-path native registrations in
`routes()` at `d9PublicWriteThrough` (restore the deleted method from
this lane's diff) and delete `api/oidc_terminate.go` with its
`api/oidc_*_test.go` proofs, `contracts/oidc-termination.md`, and
`scripts/measure-oidc-termination-latency.sh`. No data migrates, no
session or login-state dest-side exists to unwind, and no other
family moves; the generic relay answers both paths again behind
RequireAuth. Verify by re-running the lane conformance suite
against the reverted tree: the ahead-of-relay assertions go red
(the routes relay again) while every other suite stays green. No
Plan A rehearsal is consumed by this rollback — it restores a proxy
path, not durable state.
