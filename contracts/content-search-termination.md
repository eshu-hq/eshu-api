# W4 Content search relay-leg termination

W4 Content search 4, terminated (Refs `eshu-hq/eshu#9669`, satellite
`eshu-hq/eshu-api#111`). `POST /api/v0/content/files/search`,
`/entities/search`, `/dockerfile-installs/search`, and
`/php-config/search` terminate dest-side inside the W4 wave gate, ahead
of the generic proxy.

## Pin

`ContentSearchReadAdmission v0`, recorded in
`contracts/content-search-read-admission.md` at the W4 base
(`e504e328a7`, the verified core HEAD; pins minted at `330828359a`,
merge of `eshu-hq/eshu#10540`). The lane consumes only this narrow pin:
the four routes' bounded reads ride `SearchFiles`, `SearchEntities`,
`SearchFileContent`, `SearchFileContentAnyRepo`,
`SearchFilesUnscoped`, `SearchEntityContent`,
`SearchEntityContentAnyRepo`, `SearchEntitiesUnscoped`,
`SearchDockerfileInstallEntities`, and `SearchPHPConfigEntities` —
the exact 10 re-declared methods. The lane cites no other pin, and no
other family may cite this pin as coverage. The grant check (plus the
repository selectors), the fixed failure messages, `QueryProfile`, and
the hybrid/semantic rerank stay handler behavior, not port shape —
core judges them on the forwarded call.

## Serving

- Shared-key (`AllScopes`) callers terminate on the native lane:
  the family authorizer (`authorizeContentSearchSharedKey`) runs first,
  then the call crosses the versioned port transport
  (`forwardToCore` on the native lane) with method, path, query,
  body, negotiation headers, and credential intact; core bytes come
  back unmodified. Byte parity with the generic relay is pinned by
  `TestContentSearchTerminatedMatchesRelayByteForByte` across the
  fragment's documented outcomes (200 found and empty, 400
  malformed/validation, 403 grant-denied, 404, fixed-message 500,
  store-unavailable 503, retryable 503 with `Retry-After`, 504
  deadline, unfollowed redirect) in both negotiated shapes, for all
  four routes and every driven body shape (scoped, multi-repo, and
  unscoped file/entity shapes; package and directive/mode shapes).
- Credential classes dest cannot resolve relay explicitly until
  D8-grade parity, and core judges them there:
  - scoped API tokens (`Bearer [REDACTED]`, any non-key bearer),
  - OIDC bearers (`Bearer eyJ...` carrying an OIDC `iss`),
  - browser sessions (`eshu_session` / `__Host-eshu_session`
    cookies). The explicit relay carries the caller's credential,
  method, path, query, and body to the same core route untouched;
  dest invents no 401, 403, 404, or 500 for these classes
  (`TestContentSearchWaveRelayExplicitForUnresolvableIdentities`).
- Credential-less callers — and junk-cookie carriers, per the D2
  follow-up lesson — get the standing dest 401 with zero core
  contact. No public carve-out exists in this lane: the routes stay
  inside RequireAuth, keyless callers keep the 401, and no
  public-registration change rides this lane.

## D0 / wiring evidence

- `TestContentSearchWaveFrozen`: the wave is exactly the four pinned
  search POSTs; nothing else may enter it silently. The table is a NEW
  wave, never an extension of Wave-2 `contentWave`: the point-read pin
  covers point reads only, and the Wave-2 parity hygiene ban keeps
  search symbols out of the point-read files.
- `TestContentSearchWaveAheadOfRelay` (+ slash spellings): shared-key
  calls file their dispatch on the native lane ahead of the generic
  proxy, with request bodies forwarded intact across scoped,
  multi-repo, unscoped, and selector shapes.
- `TestContentSearchAuthorizerCountersPinned`: one allow judgment on
  the `content` family per terminated call; the deny branch stays
  observable (`TestContentSearchDenyBranchShapePinned` pins its 403
  shape with zero core contact).
- Resilience: dead-core single-attempt 503, pre-cancel and mid-flight
  cancellation without retry, and a 16-caller parallel probe (one
  forward per caller).
- Latency: `BenchmarkContentSearchTerminatedPOST` vs
  `BenchmarkContentSearchRelayPOST` bound the gate + authorizer
  overhead; both legs do one server-side fetch (see
  `docs/content-search-latency.md` for the measured run).
- Spec scope: the routes mount natively and serve natively; mounted
  201 -> 205, served +4, disclosed absences unchanged at 31.

## Spec posture

The four search POSTs mount natively AND serve natively in this lane:
their Decision 5(c) fragment ports entry-shaped. The pinned core file
`go/internal/query/openapi/paths/search/content.go` carries the lane
entries mid-list with separator commas, so the
`openapi/paths/search/content_search.go` subset concatenates without
re-shaping (gated line-verbatim in `scripts/check-parity.sh` reusing
`$SRC_SHA_W4`); the point-read entries already shipped with the Wave-2
lane from the same file. No disclosed absence is recorded — the W4
lane is the port lane, not a relay-leg lane like gap-04 Compare or W6a
Code-flow.

The port also vendors the schemas the search entries reference that
the document did not already carry, each line-verbatim from its pinned
core source at the same base: `EntityContentSearchResponse` (from
`go/internal/query/openapi/components_entity_search.go`, with its
`RankWalk` reference), `RankWalk` (from
`go/internal/query/openapi/components_rank_walk.go`), and the
`RankCursorConflict` response (from
`go/internal/query/openapi/components_responses.go`). `FileContent`,
`EntityContent`, and the envelope responses already shipped with
earlier lanes.

## Rollback

Revert-to-relay: remove the `s.registerContentSearchWave(mux)` call in
`routes()` (or the wave/gate/terminate/test files with this
contract), and the four search POSTs return to the generic relay
behind RequireAuth. The served fragment and its three component blocks
leave with the lane (they are the lane's port, not shared surface); no
data migrates, and no other family moves.
