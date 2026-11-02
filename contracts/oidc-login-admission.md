# OIDC login admission contract

Versioned admission-contract port for OIDC login
(`GET /api/v0/auth/oidc/login`, `GET /api/v0/auth/oidc/callback`) (Refs
`eshu-hq/eshu#8872`). The API handler keeps the HTTP shape below and
calls this port; the core-owned Service implementation satisfies it.
`eshu-api` never takes a direct table handle.

## Version

`OIDCLoginAdmissionContractVersion = "v0"`, with the port defined as:

```go
type OIDCLoginAdmission = OIDCLoginService
```

The line is vendored verbatim in
`contracts/upstream/oidc-login-wire-shapes.txt`, with the surrounding
contract comment, the `OIDCLoginService` port (2 methods:
`StartOIDCLogin`, `CompleteOIDCLogin`), the start/complete
request/response shapes, and the route registration. The frozen method
set is `frozenOIDCLoginAdmission` in
`contracts/upstream/oidc_login_admission_contract_test.go`.

Out of contract by design: the optional `OIDCProviderLister` is not part
of v0 — a dest client implementing only the 2 admission methods silently
drops runtime provider discovery and must implement the lister or accept
DB-row-only discovery. Browser-session issuance on callback belongs to
the BrowserSessions family's own seam, not this contract.

## HTTP shape

The operator-facing login flow, vendored
verbatim in `contracts/upstream/oidc-login-wire-shapes.txt`:

- Login: the start request selects one configured provider and the
  tenant/workspace boundary; the response carries the provider
  authorization redirect URL.
- Callback: the state and authorization code resolve into an Eshu auth
  context with the provider subject id, hashed group claims, and proof
  timestamp.
- Failure modes fail closed without leaking provider or claim details:
  503 when the service is unconfigured (nil service unwraps to a nil
  port, never a value adapter wrapping nil), 500 with the cause on the
  request span only.
- OIDC login activates from an env config file or explicit enablement
  for DB-only provider registration; a missing database fails the
  wiring, and disabled configs return no handler.

## Enforcement

`TestOIDCLoginAdmissionContractIsVersioned` pins the version and the
frozen method set; the wiring identity test pins the exact injected
port. A renamed, added, removed, or re-signed method fails the frozen
test until the list is deliberately updated; bump
`OIDCLoginAdmissionContractVersion` alongside (manually — the bump is
convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.
