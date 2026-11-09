# Signinpolicy-group native termination

First of the sequential termination waves (Refs `eshu-hq/eshu#9669`,
satellite `eshu-hq/eshu-api#72`, arbiter APPROVED-WITH-CONDITIONS, D9
order signinpolicy-group FIRST, one family per PR). This lane
terminates the relay legs for exactly two routes so the satellite
natively terminates them:

- `GET /api/v0/auth/sign-in-policy` (public sign-in-policy hint)
- `GET /api/v0/auth/providers` (public posture picker)

Everything else stays where it is. Core repo takes no code in this
wave.

## What termination means here

Both routes register as exact-path native-public handlers outside
every gate, ahead of the generic proxy (see `api/router.go`). Dest
synthesizes the 200 from fetched core answers; no core bytes cross
verbatim on these two paths:

- Public sign-in-policy: the `require_sso` signal comes from the
  frozen `SignInPolicyReadAdmission` v0 port
  (`signInPolicyReadPort.GetSignInPolicy`, one method — the D0 proof),
  served by a request-scoped core-fetching adapter over D0-proven
  transport (one anonymous GET of the canonical core public route, no
  retry). Dest synthesizes `{require_sso}` with
  `Cache-Control: public, max-age=30`. Empty tenant answers
  `{require_sso:false}` with zero core contact.
- Providers: the provider list comes from one anonymous fetch of the
  canonical core providers route and the `local_login_offered` bit
  comes from the same frozen port; dest synthesizes
  `{providers, local_login_offered, self_service_tokens_offered}`
  with `Cache-Control: public, max-age=60`. Empty tenant answers the
  safe zero default with zero core contact.

Dest takes no table or store handle anywhere in the lane
(`api/signinpolicy_terminate.go` references no `sql`, store, table,
or DB handle — only prose mentions of core-side store outages).

## What stays

- `GET /api/v0/auth/admin/sign-in-policy` stays wave-gated (shared-key
  forward, foreign classes relay explicitly, credential-less 401).
- `PATCH /api/v0/auth/admin/sign-in-policy` stays on its RequireAuth
  tier-C write-through.
- Trailing-slash twins, deeper subtrees, and non-GET spellings of the
  two public paths stay on the generic relay behind RequireAuth:
  keyless callers keep the standing 401 with zero core contact;
  keyed callers get core's answer for the spelled path.
- Docs/redoc shells, the D3/D4/D6 terminated waves, the D7/D8/D9
  dispatch lanes, and the setup carve-out are untouched.

## Group gates (evidence)

- D0 proof: `TestSignInPolicyPortIsFrozen` pins the dest-side frozen
  port (one method, exact signature); production calls it on both
  terminated legs; grep shows no store handle in the lane file.
- Pin byte-identical at fresh core SHA `fc6e9153` (origin/main):
  `sign_in_policy_read_admission_contract_test.go` cmp-identical;
  `sign-in-policy-read-wire-shapes.txt` excerpt fully present in
  `read_admission.go`; `openapi/paths/auth/sign_in_policy.go`
  cmp-identical to the core fragment; all 75 payload lines of
  `openapi/paths/auth/auth_providers.go` verbatim in core
  `paths/auth/routes.go`. `scripts/check-parity.sh` green.
- Public-registration exactness: `TestSigninpolicyGroup_AdminStaysAuthed`
  (admin GET/PATCH keyless 401, zero hits),
  `TestSigninpolicyGroup_PublicExactnessRED` (13 non-carved spellings
  keyless 401 with the bare Bearer challenge, zero hits),
  `TestSigninpolicyGroup_TwinsStayRelay`,
  `TestProviders_KeyedTwinRelaysNotSynthesizes`,
  `TestSigninpolicyGroup_SignInPolicyKeyedTwinRelays` (keyed twins
  relay core bytes/headers, never synthesis).
- Byte/shape parity plain+envelope vs core for enabled/disabled/
  no-provider states: `TestPublicSignInPolicy_SynthesizesRequireSSO`,
  `TestProviders_SynthesizesPosture` (both `Accept` shapes each),
  `TestSigninpolicyGroup_SynthesizedKeysAreExact` (exact key sets, no
  tenant echo, no credential leak).
- D10 lessons: Accept forwarded on every fetch
  (`TestPublicSignInPolicy_ForwardsAcceptAndQueryWithoutCredential`,
  `TestProviders_FetchesForwardNegotiationWithoutCredential`);
  no stale ServeShapes gap (no new benchmark legs added — the
  existing leafb benchmarks do not touch these paths); twin-subtree
  200 disclosed as absent by construction (exact-only registration,
  proven 401); gate RED proof present (zero-hit assertions on every
  keyless denial).
- Retry-After only on retryable 503: every dest-synthesized 503 is
  permanent (`backend_unavailable`, never `Retry-After`), including
  when the fetched core answer carried one; a core-marked retryable
  503 on a fetch maps to the permanent dest 503 (disclosed
  fail-closed, never relayed).
- OpenAPI fragment line-verbatim: unchanged fragments, parity green
  at the fresh SHA (see above); `TestSigninpolicyGroup_SpecCarriesBothOps`
  pins the assembly.
- Redirect proof: `TestPublicSignInPolicy_RedirectStripped`,
  `TestProviders_RedirectStripped` (200 with no `Location` and no
  `Set-Cookie` even when the fetch carried both).
- Telemetry: `TestSigninpolicyGroup_TerminatedFilesNoAuthorizer`
  (native `core_ok` per family, port fetches timed, no authorizer
  rows — public routes file no judgment).
- Fail-closed box: `TestPublicSignInPolicy_FailClosed`,
  `TestProviders_ListFailureFailsClosed`,
  `TestProviders_PortFailureFailsClosed`, dead-core single-attempt
  and cancel proofs, unconfigured-origin 503, empty-tenant defaults,
  V1-alias termination for both paths.

## Rollback

Revert-to-relay: delete the two exact-path native registrations in
`routes()` and restore the `default` branch in the wave-public loop
to forward the two data reads. No data migrates and no other family
moves; the generic relay answers both paths again behind RequireAuth.
