# D5 wiring-probes contract

Versioned wiring-probe record for the D5 wave (Refs
`eshu-hq/eshu#9669`, part of `eshu-hq/eshu#6707`). This lane terminates
nothing and ports no OpenAPI fragments: it probes the wiring of the
D2 Leaf B, D3 admin/identity/recovery, and D4 Native-next terminated
GETs against what the router serves and what the served `openapi.json`
promises — methods, paths, operationIds, response shapes — and fails
on any drift the lane contracts do not disclose.

Scope source: `api/leafb_wave.go`, `api/d3_wave.go`, and
`api/d4_wave.go` at this lane's base, read directly (the probe derives
its 58-GET surface from those tables; no second mirror to drift).
Excluded and staying relay by design (not drift): every POST/DELETE/
PATCH the waves terminate — this probe watches GET wiring only; the
owning lanes' conformance suites pin the other methods.

## Status

Probe claims stay provisional with the termination claims they watch
until the arbiter re-reviews the D0 evidence (blocker B1). What this
lane proves per the D-wave ruling is the required proof list for a
probe-only lane: the frozen probe surface asserted first (strict
bijection ran RED on exactly the four disclosed absences, green with
them pinned), byte-identity of every ported line still riding
`scripts/check-parity.sh`, no-core-import (this lane adds no imports
beyond the test's stdlib use), and this contract section with
rollback. There is no authorizer, latency, or telemetry surface to
prove: the lane ships no handlers.

## Production binding

None. This lane adds no routes, no spec fragments, no latency
surface, and no telemetry: `api/d5_wiring_test.go` reads the served
document through the existing `openapi.json` route exactly as a keyed
caller would, with both a configured and an unconfigured core origin
(the document serves with no core fetch). Serving behavior is
unchanged; only the watching is new.

## Per-wave record

Scope-honesty label (inherited, unchanged): the dest authorizer
covers the shared-key (AllScopes) identity only; scoped tokens, OIDC
bearers, and browser sessions relay explicitly until the D8 session
resolver lands. This probe does not re-pin gating — the owning lanes'
auth suites do — it pins that the documented GET surface matches the
terminated GET surface.

| Wave | Terminated GETs | Served spec get-entries | Absent by disclosure |
| --- | --- | --- | --- |
| D2 Leaf B | 12 | 9 (`openapi.json` self, capabilities, service intelligence, providers, 2 sign-in-policy, 3 provider-config) | 3 (`/health` mux probe, `/docs` + `/redoc` compiled shells) |
| D3 admin/identity/recovery | 8 | 8 (6 admin-identity, 2 governance-audit) | 0 |
| D4 Native-next | 38 | 37 | 1 (repositories story: entry waits for the `schema.EvidenceBoundaries` helper port) |
| Total | 58 | 54 | 4 |

Every probed entry pins its path key, its `get` method, its promised
`operationId` (54 pinned values in `d5ProbeGETs`), and a `200`
response entry. The reverse probe (`TestD5Wiring_NoUndocumentedGETs`)
closes the loop: every `get` operation the served document promises
maps to a probed terminated GET, so the document can never promise
what the probe does not watch.

Known divergences (disclosed, not hidden — the entire drift the
probe certifies):

- The served `openapi.json` omits the repositories story key while
  the story route terminates natively (see
  `contracts/d4-native-next.md`).
- The served `openapi.json` carries no `/health`, `/docs`, or
  `/redoc` path keys: `/health` answers the dest probe shape and the
  doc shells serve compiled HTML publicly (see
  `contracts/leafb-native-now.md`; shell serving pinned by
  `TestLeafBPublicDocUI`, not by this probe).

## Spec posture

This lane ports no fragments and adds no markers: the posture test
(`TestSetupSpecPosture_StaysCoreSide`) is untouched. The next lane
that ports another public-marked fragment revisits that test again
instead of landing silently.

## Rollback

Single-commit revert: delete `api/d5_wiring_test.go` and this file.
Zero serving change (no routes, no fragments, no state, no data
migration). Verify by re-running the suite: the D2-D4 lane suites
stay green while the wiring watch is gone. No Plan A rehearsal is
consumed by this rollback.
