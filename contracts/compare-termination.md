# Compare relay-leg termination

Gap-04 deliberate later pin, terminated (Refs `eshu-hq/eshu#9669`,
satellite `eshu-hq/eshu-api#72`). `POST
/api/v0/compare/environments` terminates dest-side inside the
Compare wave gate, ahead of the generic proxy.

## Pin

`CompareReadAdmission v0`, recorded in
`contracts/compare-read-admission.md` at the core merge
(`eshu-hq/eshu#10425`). The lane consumes only this narrow pin:
the route's workload and environment-snapshot graph reads ride
`Run`/`RunSingle` and its service-evidence content reads ride
`ListRepoFiles`/`GetFileContent`. The lane cites no shared-port
pin, and no other family may cite this pin as coverage. The grant
check (`RepositoryAccessFilterFromContext` plus the repo grant
comparison) and the fixed failure messages stay handler behavior,
not port shape — core judges them on the forwarded call.

## Serving

- Shared-key (`AllScopes`) callers terminate on the native lane:
  the family authorizer (`authorizeCompareSharedKey`) runs first,
  then the call crosses the versioned port transport
  (`forwardToCore` on the native lane) with method, path, query,
  body, negotiation headers, and credential intact; core bytes come
  back unmodified. Byte parity with the generic relay is pinned by
  `TestCompareTerminatedMatchesRelayByteForByte` across the
  fragment's documented outcomes (200, 400, 403, 404, fixed-message
  500, graph-deadline 504, retryable 503 with `Retry-After`,
  unfollowed redirect) in both negotiated shapes.
- Credential classes dest cannot resolve relay explicitly until
  D8-grade parity, and core judges them there:
  - scoped API tokens (`Bearer sc_...`, any non-key bearer),
  - OIDC bearers (`Bearer eyJ...` carrying an OIDC `iss`),
  - browser sessions (`eshu_session` / `__Host-eshu_session`
    cookies).
  
  The explicit relay carries the caller's credential, method, path,
  query, and body to the same core route untouched; dest invents no
  401, 403, 404, or 500 for these classes
  (`TestCompareWaveRelayExplicitForUnresolvableIdentities`).
- Credential-less callers — and junk-cookie carriers, per the D2
  follow-up lesson — get the standing dest 401 with zero core
  contact. No public carve-out exists in this lane: the route stays
  inside RequireAuth, keyless callers keep the 401, and no
  public-registration change rides this lane.

## D0 / wiring evidence

- `TestCompareWaveFrozen`: the wave is exactly the one pinned
  Compare POST; nothing else may enter it silently.
- `TestCompareWaveAheadOfRelay` (+ slash spelling): a shared-key
  call files its dispatch on the native lane ahead of the generic
  proxy, with the request body forwarded intact.
- `TestCompareAuthorizerCountersPinned`: one allow judgment on the
  `compare` family per terminated call; the deny branch stays
  observable (`TestCompareDenyBranchShapePinned` pins its 403
  shape with zero core contact).
- Spec scope: the route is a mounted native operation; mounted
  173 -> 174, disclosed absences 10 -> 11.

## Spec posture

The Compare POST mounts natively, but its Decision 5(c) fragment
stays core-side for this lane: core shapes
`go/internal/query/openapi/paths/status/compare.go` as the
paths-closing entry of its own assembled document (last in core
assembly, trailing `},`), so a byte-identical satellite port cannot
concatenate mid-list — and a re-shaped satellite subset would break
the verbatim-port rule `scripts/check-parity.sh` gates. The route
is therefore a disclosed spec absence (same posture the setup lane
records for its core-side fragments), pinned by
`TestSpec_CompareStaysDisclosedAbsent`. A follow-up spec-port lane
claims the key when core re-shapes the fragment or the lane ports
it entry-shaped; until then the served document is unchanged by
this lane.

## Rollback

Revert-to-relay: delete the Compare wave loop in `routes()` (and
its wave/gate/terminate/test files with this contract). The
Compare POST returns to the generic relay behind RequireAuth and
rejoins the D6 disclosed relay lists; the spec absence stays
disclosed either way. No data migrates and no other family moves.
