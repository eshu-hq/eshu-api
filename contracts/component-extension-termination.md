# Component-extensions satellite termination

Satellite termination of the 2 component-extension reads (Refs
`eshu-hq/eshu#9669`, satellite `eshu-hq/eshu-api#101`, Wave-1b ruling:
PROCEED on both legs). The wave table lives in
`api/compext_wave.go`; the serving contract in
`api/compext_terminate.go`; the router registration in
`api/router.go` (`registerCompextWave`, ahead of the generic
proxy). The scope-honesty label rides this record: the dest
authorizer covers the shared-key (AllScopes) identity only, and core
re-judges every forwarded call as backstop (tenant scope, permission
catalog, governance mode — where the live 403s come from).

## Pins

One deliberate core pin, recorded byte-identical at the core HEAD
`9bf29e53` and verified in-lane with zero drift since its mint
(`git diff 38954799bb origin/main` empty on the pin file). The
vendored `.go` port plus the frozen method-set contract test are
whole-file byte-identical at that HEAD (gated in
`scripts/check-parity.sh`); the `-read-wire-shapes.txt` excerpt
carries the version-const stanza, the interface, and the in-scope
route list, line-verbatim from the pin file at the same HEAD.

| Pin | Record | Upstream mirrors | Methods | Covers |
| --- | --- | --- | --- | --- |
| `ComponentExtensionReadAdmission v0` | `contracts/component-extension-read-admission.md` | `contracts/upstream/component_extension_read_admission.go` (`6ee1e468…`), `contracts/upstream/component_extension_read_admission_contract_test.go` (`601c577e…`) | 1 | `GET /api/v0/component-extensions`, `GET /api/v0/component-extensions/{component_id}/diagnostics` |

The lane consumes only this pin and cites no shared-port pin; no
other family may cite this pin as coverage. The list-leg limit
truncation, the diagnostics-leg `component_id` filter + 404, grant
checks, fixed failure messages, and QueryProfile are handler behavior
OUT of the pin.

## FS conditions (carried forward from the ruling, unchanged)

- Dest never reads a dest-local `ESHU_COMPONENT_HOME`; core reads its
  own registry on the forwarded call. (`TestCompextTerm_LaneFileHygiene`
  fails the lane on any `ESHU_COMPONENT_HOME` sighting.)
- Parity drives all three branches: home unconfigured (503), home
  configured with components (200 + diagnostics 200/404), and
  policy-gated rows.
- `Native` here means dest-owned gate/authorizer over a core-local FS
  read.

## Census (per route per method)

Every wave route admits its read through exactly the pinned method
named below.

| Route | Pin method |
| --- | --- |
| `GET /api/v0/component-extensions` | `Readback` |
| `GET /api/v0/component-extensions/{component_id}/diagnostics` | `Readback` |

## Relay posture

Shared-key callers terminate on the native lane; foreign credential
classes (scoped token, OIDC bearer, session cookie) relay explicitly
on the relay lane so core judges them; credential-less callers — and
junk-cookie carriers — keep the standing dest 401 with zero core
contact. The only dest-synthesized errors on the terminated path are
the gate 401, the future-resolver family 403, and the permanent local
503 when no core origin is configured (never Retry-After). Rollback
is revert-to-relay: remove the `registerCompextWave` call and the 2
routes return to the generic relay behind RequireAuth.
