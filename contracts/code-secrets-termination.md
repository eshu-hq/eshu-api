# Code-secrets relay-leg termination

W6c code-secrets 1, terminated (Refs `eshu-hq/eshu#9669`, satellite
`eshu-hq/eshu-api#98`). `POST
/api/v0/code/security/secrets/investigate` terminates dest-side
inside the W6c secrets wave gate, ahead of the generic proxy.

## Pin

`CodeHardcodedSecretReadAdmission v0`, recorded in
`contracts/code-secrets-read-admission.md` at the W6 remainder mint
(`81b756fd84`, the verified core merge of `eshu-hq/eshu#10688`).
The lane consumes only this narrow pin: the plain investigation
(`InvestigateHardcodedSecrets`) plus the source-reporting twin
(`InvestigateHardcodedSecretsWithSource`) — the exact 2 re-declared
methods, with `Run`, `RunSingle`, and every other graph and content
leg sealed OUT. The lane cites no other pin, and no other family may
cite this pin as coverage.

## Census (per route per method)

| Method | `POST /api/v0/code/security/secrets/investigate` |
|---|---|
| `InvestigateHardcodedSecrets` | fallback path on a plain-only store (side-table source default) |
| `InvestigateHardcodedSecretsWithSource` | preferred path; labels the legacy-scan read path when the secret-line index is not published ready |

Reachability notes, recorded not silently assumed: the route prefers
the `WithSource` twin and tolerates the plain-only store; the
legacy-scan annotation rides the response, never the pin; a blocked
(empty) grant answers the empty redacted-findings page with no read;
the findings always carry the
`secret_values_replaced_with_redacted_marker` redaction — raw
secret-line text never crosses this pin.

## Serving

- Shared-key (`AllScopes`) callers terminate on the native lane:
  the family authorizer (`authorizeCodeSecretsSharedKey`) runs
  first, then the call crosses the versioned port transport
  (`forwardToCore` on the native lane) with method, path, query,
  body, negotiation headers, and credential intact; core bytes come
  back unmodified. Byte parity with the generic relay is pinned by
  `TestCodeSecretsTerminatedMatchesRelayByteForByte` across the
  documented outcomes (side-table and legacy-scan 200s with the
  source labels and redaction, blocked empty-grant page, 400
  validation/selector/kind denials, 403 grant-denied, capability
  501, fixed-message 500, configuration-state and retryable 503s
  with `Retry-After`, gateway 504) in both negotiated shapes, for
  the route and every driven body shape (kind-scoped and unscoped
  investigations).
- Credential classes dest cannot resolve relay explicitly until
  D8-grade parity, and core judges them there:
  - scoped API tokens (`Bearer [REDACTED]`, any non-key bearer),
  - OIDC bearers (`Bearer eyJ...` carrying an OIDC `iss`),
  - browser sessions (`eshu_session` / `__Host-eshu_session`
    cookies). The explicit relay carries the caller's credential,
  method, path, query, and body to the same core route untouched;
  dest invents no 401, 403, 404, 409, 422, or 500 for these classes
  (`TestCodeSecretsWaveRelayExplicitForUnresolvableIdentities`).
- Credential-less callers — and junk-cookie carriers, per the D2
  follow-up lesson — get the standing dest 401 with zero core
  contact. No public carve-out exists in this lane: the route stays
  inside RequireAuth, keyless callers keep the 401, and no
  public-registration change rides this lane.
- Decision 2: NOT OWED in this lane. There is no tier-C mutation on
  the W6c surface — the wave entry is a read-only POST, and the
  parity corpus carries no mutation outcome
  (`TestCodeSecretsDecision2NotOwed` pins the read-only wave and
  the read-outcome status set).

## D0 / wiring evidence

- `TestCodeSecretsWaveFrozen`: the wave is exactly the one pinned
  W6c POST; nothing else may enter it silently. The table is a NEW
  wave, never an extension of `codeGraphWave`, `deadCodeWave`,
  `codeDivergenceWave`, `relationshipWave`, `codeScanWave`, or
  `codeInvestigationWave`: those pins cover their own rows only.
  Consuming this pin for any other route is a NO-GO (arbiter
  supplemental ruling, §2C).
- `TestCodeSecretsWaveAheadOfRelay` (+ slash spellings):
  shared-key calls file their dispatch on the native lane ahead of
  the generic proxy, with request bodies forwarded intact across
  kind-scoped and unscoped shapes. Removing the
  `registerCodeSecretsWave` call returns the route to the generic
  relay, which this test pins as RED (relay row, no native row).
- `TestCodeSecretsAuthorizerCountersPinned`: one allow judgment
  on the `code_secrets` family per terminated call; the deny
  branch stays observable
  (`TestCodeSecretsDenyBranchShapePinned` pins its 403 shape with
  zero core contact).
- Resilience: dead-core single-attempt 503, pre-cancel and
  mid-flight cancellation inherit the shared dispatch floor (the lane
  adds no retry), and each leg forwards exactly once per caller.
- Spec scope: the route mounts natively, but its Decision 5(c)
  fragment stays core-side for this lane; mounted 227 -> 228,
  disclosed absences 53 -> 54.

## Spec posture

The W6c POST mounts natively, but its Decision 5(c) fragment stays
core-side for this lane. The entry lives in one core file —
`go/internal/query/openapi/paths/code/security.go` (the
`/code/security/secrets/investigate` entry). Porting the entry
without its referenced schemas would break the verbatim-port rule
`scripts/check-parity.sh` gates — so, like the gap-04 Compare, W6a
Code-flow, W4 search, G-a Code-graph, Wave-4 Drift, dead-code,
divergence, G-b, W6b, and W6c investigation relay-leg lanes, the
route is a disclosed spec absence, not a port. A follow-up spec-port
lane claims the key when it ports the entry entry-shaped with its
referenced schemas at a pinned base; until then the served document
is unchanged by this lane.

## Rollback

Revert-to-relay: remove the `s.registerCodeSecretsWave(mux)`
call in `routes()` (or the wave/gate/terminate/test files with this
contract), and the W6c POST returns to the generic relay behind
RequireAuth. The spec absence stays disclosed either way (the route
leaves the mounted set with the lane); no data migrates, and no other
family moves.
