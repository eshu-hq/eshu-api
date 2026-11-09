# GitHub-pair native termination

Third of the sequential termination waves (Refs `eshu-hq/eshu#9669`,
satellite `eshu-hq/eshu-api#72`, arbiter APPROVED-WITH-CONDITIONS, D9
order GitHub after the OIDC merge, one family per PR).
This lane terminates the relay legs for exactly two routes so the
satellite natively terminates them:

- `GET /api/v0/auth/github/login` (public login start)
- `GET /api/v0/auth/github/callback` (public login callback)

OIDC stays on its own landed termination lane and SAML stays relay;
SAML is never named outside the boundary tests that pin it untouched.
Core repo takes no code in this wave.

## What termination means here

Both routes register as exact-path native-public handlers outside
every gate, ahead of the generic proxy (see `api/router.go`). Dest
owns the route mapping over the versioned `GitHubLoginAdmission` v0
port (`githubLoginPort.StartGitHubLogin` + `CompleteGitHubLogin`, the
D0 proof): each terminated GET makes exactly one anonymous core fetch
over D0-proven transport with the caller query crossing as opaque
bytes, and dest synthesizes the answer from the port result — no
core bytes cross the generic relay on these two paths. Unlike OIDC
this is plain OAuth2 (no discovery document, no ID token, no JWKS,
no nonce); the OAuth code exchange plus session mint stay core-side
(the D8 resolver stays complete downstream: callback issuance keeps
its BrowserSession shape, and this lane mints none itself).

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
  revoked grants, or the IdP-denied error params core maps to its
  denial envelope) create no session because core created none.
- Exactly-once stays core-side (`ConsumeState`): a parallel
  duplicate callback makes one fetch per delivery — dest holds no
  lock and serializes nothing — the first wins one session and every
  loser relays core's error.

Dest takes no table or store handle anywhere in the lane
(`api/github_terminate.go` references no `sql`, store, table, or DB
handle — only prose mentions of core-side stores), mints no session,
keeps no replay cache, appends no audit event, and runs no finalize.
Governance audit stays core-side: the identity-authentication event
(`sso_login_authenticated`, or a denied classification such as
`org_not_allowed` carrying the allow-list reason) emits from inside
the core callback handler that still serves every fetch.

## What stays

- `GET /api/v0/auth/oidc/login` + `GET /api/v0/auth/oidc/callback`
  stay on the OIDC termination lane; the SAML triple stays on the
  SAML write-through lane. Neither is named outside the boundary
  tests that pin them untouched.
- Trailing-slash twins, deeper subtrees, and non-GET spellings of the
  two GitHub paths stay on the generic relay behind RequireAuth:
  keyless callers keep the standing 401 there with zero core contact;
  keyed callers get core's answer for the spelled path.
- No admin route moves in this lane: the admin provider-config reads
  and every other authed family stay on their existing lanes
  (parallel-safe: the router hunk touches only the two GitHub exact
  paths with no shared lines; this lane lands only after the group-2
  merge).
- Docs/redoc shells, the leafb/D3/D4/D6 terminated waves, the D7/D8
  dispatch lanes, the setup carve-out, and the signinpolicy-group
  termination are untouched.

## Group gates (evidence)

- D0 proof: `TestGitHubTermPortIsFrozen` pins the dest-side frozen
  port (two methods, exact frozen signatures); production calls it on
  both terminated legs; `TestGitHubTerm_LaneFileHygiene` greps no
  store/table/lock primitive in the lane file.
- Pin byte-identical: `GitHubLoginAdmission` v0 at core SHA
  `8c2324693320c8f69ef3e8078c795956b549d7ad` (see
  `contracts/d9-github-login.md` + `SRC_SHA_D9_GITHUB` in
  `scripts/check-parity.sh`, unchanged by this lane); D8 session
  pins complete at the re-anchored SHA (BrowserSession shape carries
  the callback cookies — this lane mints none itself).
- Public-registration exactness: `TestGitHubTermGroup_PublicExactnessRED`
  (10 non-carved spellings keyless 401, zero hits),
  `TestGitHubTermGroup_KeyedExactIsNative` (keyed exact paths ride
  the terminated lane: 302 + byte-verbatim Location + one fetch of the
  canonical core route per leg), `TestGitHubTermGroup_KeyedTwinRelays` (keyed twins
  relay core bytes for the spelled path, never synthesized),
  `TestGitHubTermGroup_V1AliasTerminates` (both v1 spellings
  terminate).
- Byte parity terminated-vs-relay for the full D9 outcome matrix
  (start 302 incl. duplicate, callback 201/303 with cookies,
  unconfigured 503, validation 400, callback 401, denied 403, core
  500, core 503 retryable/permanent) in both negotiated shapes:
  `TestGitHubTerm_MatchesRelayByteForByte` (status, body, Location,
  Retry-After, Content-Type, Set-Cookie per key in order).
- Redirect proof: `TestGitHubTerm_RedirectUnfollowed` (302 relays its
  Location, exactly one upstream hit),
  `TestGitHubTerminated_LoginSynthesizesRedirect` /
  `TestGitHubTerminated_CallbackSynthesizesSession` (redirect +
  session cookies byte-verbatim, one fetch each).
- Error + disabled-provider deny shape: `TestGitHubTerm_NoRetryOnUnstable`
  (500/retryable-503/permanent-503 verbatim, one attempt each),
  `TestGitHubTerm_DisabledProviderDenyShape` (unavailable 503
  verbatim), `TestGitHubTerm_OAuthErrorParamShape` (IdP-denied
  error params cross opaque; core's mapped 403 envelope verbatim,
  one fetch), `TestGitHubTerm_DeadCoreSingleAttempt` +
  `TestGitHubTerm_UnconfiguredOriginFailsClosed` (permanent local
  503, never `Retry-After`).
- Exactly-once replay under parallel duplicate callback: one session,
  loser core error — `TestGitHubTerm_ParallelDuplicateCallbackExactlyOnce`
  (16 parallel deliveries, one 201, fifteen 403s, 16 fetches, never
  serialized — proved, plus the no-`sync`/`Mutex` grep).
- Opaque-bytes callback: `TestGitHubTermNative_CredentialHygiene`
  (state/code/Location/cookie canaries absent from logs and
  metrics), `TestGitHubTerm_LaneFileHygiene` (no `.Query(`,
  `ParseForm(`, `FormValue(`, `.RequestURI`, credential literals,
  logger/tracer, store, or lock in the lane file),
  `TestGitHubTerm_KeylessCarriesNoCredential` (no shared-key
  injection; caller keys cross untouched per the D9 pin).
- OpenAPI fragments line-verbatim: unchanged fragments, parity
  green at the pinned SHA (see above);
  `TestGitHubTermGroup_SpecCarriesBothOps` pins the assembly.
- Telemetry: `TestGitHubTerm_TerminatedFilesNoAuthorizer`
  (native `core_ok` per leg, port fetches timed, no authorizer
  rows — public routes file no judgment).
- 5-leg latency: `scripts/measure-github-termination-latency.sh`
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
`routes()` at `d9GitHubPublicWriteThrough` (restore the deleted
method from this lane's diff) and delete `api/github_terminate.go`
with its `api/github_*_test.go` proofs,
`contracts/github-termination.md`, and
`scripts/measure-github-termination-latency.sh`. No data migrates,
no session or login-state dest-side exists to unwind, and no other
family moves; the generic relay answers both paths again behind
RequireAuth. Verify by re-running the lane conformance suite
against the reverted tree: the ahead-of-relay assertions go red
(the routes relay again) while every other suite stays green. No
Plan A rehearsal is consumed by this rollback — it restores a proxy
path, not durable state.
