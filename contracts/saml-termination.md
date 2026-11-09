# SAML-triple native termination

Last of the sequential termination waves (Refs `eshu-hq/eshu#9669`,
satellite `eshu-hq/eshu-api#72`, arbiter APPROVED-WITH-CONDITIONS, D9
order SAML after the GitHub merge, one family per PR). This lane
terminates the relay legs for exactly three routes so the satellite
natively terminates them:

- `GET /api/v0/auth/saml/providers/{provider_id}/metadata` (public
  metadata read)
- `GET /api/v0/auth/saml/providers/{provider_id}/login` (public
  login start)
- `POST /api/v0/auth/saml/providers/{provider_id}/acs` (public ACS
  consume)

OIDC and GitHub stay on their landed termination lanes; this lane
never names them. Core takes no code in this wave.

## What termination means here

All three routes register as exact-path native-public handlers
outside every gate, ahead of the generic proxy (see `api/router.go`,
lane-owned selector `samlTerminatedHandler` in
`api/saml_terminate.go` so `routes()` stays under the file-size cap).
Dest owns the route mapping over the versioned `SAMLLoginAdmission`
v0 port (`samlLoginPort`: `FetchSAMLMetadata` + `StartSAMLLogin` +
`CompleteSAMLLogin`, the D0 proof): each terminated leg makes exactly
one anonymous core fetch over D0-proven transport with the caller
bytes crossing as opaque bytes, and dest synthesizes the answer from
the port result — no core bytes cross the generic relay on these
three paths.

- Metadata: the 200 answer carries the provider-specific EntityID and
  POST ACS endpoint XML byte-verbatim, with the
  `application/samlmetadata+xml` content type verbatim. Failures fail
  closed without leaking provider or claim details: the
  unknown-provider 404 relays core bytes verbatim; the disabled-store
  503 relays core bytes verbatim; only the unreachable/unconfigured
  core answers the dest-synthesized permanent local 503
  (`backend_unavailable`, never `Retry-After`).
- Login start: the 302 answer carries the provider IdP SSO `Location`
  byte-verbatim — SAMLRequest, RelayState, SigAlg, and Signature
  intact. Failures fail closed the same way: the unknown-provider
  404, the 400 invalid, and the 403 denied shapes relay core bytes
  verbatim; only the unreachable/unconfigured core answers the
  dest-synthesized permanent local 503.
- ACS: the 201 answer carries the browser-session JSON plus the two
  `Set-Cookie` session cookies byte-verbatim
  (name/value/attrs/expiry); the 303 answer carries both cookies
  before redirecting to the stored same-origin return path, with its
  `Location` byte-verbatim. Denied ACS deliveries (400 missing
  fields, 401 spent or unknown request, 403 denied principal) create
  no session because core created none. The consumed, unknown, or
  invalid request relays core bytes verbatim — a spent request answers
  401 and the user restarts login.
- Exactly-once stays core-side (consume plus replay reservation): a
  parallel duplicate ACS makes one fetch per delivery — dest holds no
  lock and serializes nothing — the first wins one session and every
  loser relays core's spent-request 401.
- Opaque bytes, binding: the provider path value crosses whole
  (`EscapedPath`, never segmented into a variable), the `return_to`
  query value crosses byte-identical and never parsed, and the ACS
  form (`RelayState` + `SAMLResponse`) crosses as bounded bytes, never
  decoded — the dest shared key is never injected, and no path value,
  query value, form value, RequestURI, Location, or Set-Cookie value
  reaches logs, metrics, or spans. The ACS mutation is never retried
  and never serialized dest-side.
- Size-bounded ACS: dest mirrors core's own 1MiB cap at the lane file
  (`samlTermMaxBody`), for the forwarded form and for every mapped
  answer. A near-1MiB form forwards byte-intact with one attempt
  (core judges); an over-1MiB form fails closed to the permanent
  local 503 with zero upstream contact. An unreadable body fails
  closed the same way.
- No dest-side SAML client ever: no store lookup, no verifier, no key
  material of any kind — the SP key material (service-provider
  private key, IdP metadata) never leaves core. In particular the lane
  file never sights `GetSAMLProvider`: that store method returns the
  SP key material, so the dest-side metadata call is deliberately
  named `FetchSAMLMetadata` instead of mirroring it, and
  `scripts/check-parity.sh` fails any `GetSAMLProvider` sighting in
  the lane file. Dest holds no SAML state, session, or provider
  material, mints no credential or idempotency value, keeps no replay
  cache, appends no audit event, and runs no finalize.
- Wildcard mux contract: the `{provider_id}` wildcard matches any
  single segment, so unknown provider IDs forward opaquely and relay
  core's 404 verbatim; multi-segment spellings and deeper subtrees
  match no route and keep the standing 401 with zero core contact.

Dest takes no table or store handle anywhere in the lane
(`api/saml_terminate.go` references no `sql`, store, table, or DB
handle — only prose mentions of core-side stores), mints no session,
keeps no replay cache, appends no audit event, and runs no finalize.
Governance audit stays core-side: the identity-authentication event
emits from inside the core login and ACS handlers that still serve
every fetch.

## What stays

- `GET /api/v0/auth/oidc/login` + `GET /api/v0/auth/oidc/callback`
  stay on the OIDC termination lane; `GET /api/v0/auth/github/login`
  + `GET /api/v0/auth/github/callback` stay on the GitHub termination
  lane. Neither is named outside the boundary tests that pin them
  untouched.
- Trailing-slash twins, deeper subtrees, and non-GET spellings of the
  metadata/login paths (plus non-POST spellings of the ACS path) stay
  on the generic relay behind RequireAuth: keyless callers keep the
  standing 401 there with zero core contact; keyed callers get core's
  answer for the spelled path.
- The empty-provider spelling (`.../providers//login`) still never
  reaches a route: Go's mux sanitizes the doubled slash with a
  redirect to the cleaned path, and the cleaned path matches no
  carved route. The escaped-slash spelling (`.../providers/a%2Fb/...`)
  still matches the wildcard as one segment and forwards opaquely.
- HEAD on the metadata/login GETs still routes to the GET pattern and
  forwards once for core to judge; dest adds no HEAD branch.
- Docs/redoc shells, the leafb/D2/D3/D4/D6 terminated waves, the D7/D8
  dispatch lanes, the setup carve-out, and the signinpolicy-group
  termination are untouched (parallel-safe: the router hunk is the
  SAML block only with no shared lines; this lane lands only after
  the group-3 merge).

## Group gates (evidence)

- D0 proof: `TestSAMLTermPortIsFrozen` pins the dest-side frozen
  port (three methods, exact frozen signatures, deliberately not
  naming the SP-key-material lookup); production calls each method on
  its terminated leg; `TestSAMLTerm_LaneFileHygiene` greps no
  store/table/lock primitive and no `GetSAMLProvider` sighting in the
  lane file, and `scripts/check-parity.sh` gates the same negative
  list.
- Pin byte-identical: `SAMLLoginAdmission` v0 at core SHA
  `8c2324693320c8f69ef3e8078c795956b549d7ad` (see
  `contracts/d9-saml-login.md` + `SRC_SHA_D9_SAML` in
  `scripts/check-parity.sh`, unchanged by this lane); D8 session
  pins complete at the re-anchored SHA (BrowserSession shape carries
  the ACS cookies — this lane mints none itself).
- Public-registration exactness: `TestSAMLTermGroup_PublicExactnessRED`
  (13 non-carved spellings keyless 401, zero hits),
  `TestSAMLTermGroup_KeyedExactIsNative` (keyed exact paths file
  `via="native"`), `TestSAMLTermGroup_KeyedTwinRelays` (keyed twins
  relay core bytes for the spelled path, never synthesized),
  `TestSAMLTermGroup_V1AliasTerminates` (all three v1 spellings
  terminate).
- Byte parity terminated-vs-relay for the full SAML outcome matrix
  (metadata 200 incl. unknown-provider 404, login 302 incl.
  SigAlg/Signature verbatim, duplicate 302, refused 404, ACS 201/303
  with cookies, duplicate/spent 401, refused 400, core 500, core 503
  retryable/permanent) in both negotiated shapes:
  `TestSAMLTerm_MatchesRelayByteForByte` (status, body, Location,
  Retry-After, Content-Type, Set-Cookie per key in order).
- Redirect proof: `TestSAMLTerm_RedirectUnfollowed` (302 relays its
  Location, exactly one upstream hit),
  `TestSAMLTerminated_MetadataSynthesizesXML` /
  `TestSAMLTerminated_LoginSynthesizesRedirect` /
  `TestSAMLTerminated_ACSSynthesizesSession` /
  `TestSAMLTerm_ACS303CarriesCookiesBeforeRedirect` (XML type+bytes,
  redirect incl. SigAlg/Signature, session cookies, post-login
  redirect with cookies before Location — one fetch each).
- Swap-mutant lesson: every per-leg test drives a fresh stub core and
  asserts the upstream method, path (whole, wildcard substituted),
  query, and body — a leg swap that changes behavior (any swap
  involving the ACS leg, which alone carries a body) fails on the
  body/method assertions. The metadata/login pair is
  transport-identical by construction (two anonymous GETs of the
  caller path — the carried bytes cannot differ), so that permutation
  is the equivalent mutant: the distinction is the named port method,
  pinned frozen and reviewed.
- Wildcard mux-collision proof: `TestSAMLTerm_UnknownProviderForwards`
  (unknown IDs forward opaquely, core 404 verbatim, upstream path
  carries the unknown ID),
  `TestSAMLTerm_SiblingPathsStayRelayed` (multi-segment and deeper
  subtrees keep the standing 401, zero hits) —
  plus the unchanged dispatch proofs for empty-provider cleaning,
  escaped-slash agreement, and HEAD-on-GET.
- Error + disabled-provider deny shape: `TestSAMLTerm_NoRetryOnUnstable`
  (500/retryable-503/permanent-503/form-400 verbatim, one attempt per
  leg), `TestSAMLTerm_DisabledProviderDenyShape` (unavailable 503
  verbatim), `TestSAMLTerm_DeadCoreSingleAttempt` +
  `TestSAMLTerm_UnconfiguredOriginFailsClosed`
  (permanent local 503, never `Retry-After`).
- Exactly-once replay under parallel duplicate ACS: one session,
  loser core 401 — `TestSAMLTerm_ParallelDuplicateACSExactlyOnce`
  (16 parallel deliveries, one 201, fifteen 401s, 16 fetches, never
  serialized — proved, plus the no-`sync`/`Mutex` grep).
- Opaque-bytes ACS: `TestSAMLTermNative_CredentialHygiene`
  (provider/RelayState/SAMLResponse/Location/cookie canaries absent
  from logs and metrics, form+path byte-exact upstream),
  `TestSAMLTerm_LaneFileHygiene` (no `.Query(`, `ParseForm(`,
  `FormValue(`, `PostForm(`, `MultipartForm(`, `PathValue(`,
  `.RequestURI`, credential literals, logger/tracer, store, or lock
  in the lane file — `ReadAll` over a `LimitReader` is the one
  deliberate size-bound exception),
  `TestSAMLTerm_KeylessCarriesNoCredential` (no shared-key
  injection; caller keys cross untouched per the D9 pin).
- Body bound: `TestD9SAMLNative_LargeFormBodyStaysOpaque` (near-1MiB
  forwards byte-intact one attempt; over-1MiB fails closed 503 with
  zero upstream contact).
- OpenAPI fragments line-verbatim: unchanged fragments, parity
  green at the pinned SHA (see above);
  `TestSAMLTermGroup_SpecCarriesTripleOps` pins the assembly.
- Telemetry: `TestSAMLTerm_TerminatedFilesNoAuthorizer`
  (native `core_ok` per leg, port fetches timed, no authorizer
  rows — public routes file no judgment).
- 5-leg latency: `scripts/measure-saml-termination-latency.sh`
  (metadata, login bare, login full-query vs the GET relay leg;
  login-query vs the empty POST relay leg; ACS with the representative
  288KiB form vs the same-bytes large POST relay leg; 1.50 pathology
  bound). Parity proof runs in isolation AND under the full
  parallel race+coverage load.
- Fail-closed box: unconfigured origin, dead core single attempt,
  unreadable/oversize ACS body, unbuildable fetch — all permanent
  local 503, never `Retry-After`.

## Rollback

Revert-to-relay: re-point the three exact-path native registrations
in `routes()` at `d9SAMLPublicWriteThrough` (restore the deleted
method from this lane's diff) and delete `api/saml_terminate.go`
with its `api/saml_*_test.go` proofs, `contracts/saml-termination.md`,
and `scripts/measure-saml-termination-latency.sh`. No data migrates,
no session or login-state dest-side exists to unwind, and no other
family moves; the generic relay answers all three paths again behind
RequireAuth. Verify by re-running the lane conformance suite
against the reverted tree: the ahead-of-relay assertions go red
(the routes relay again) while every other suite stays green. No
Plan A rehearsal is consumed by this rollback — it restores a proxy
path, not durable state.
