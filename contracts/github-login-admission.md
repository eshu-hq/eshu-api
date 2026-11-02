# GitHub login admission contract

Versioned admission-contract port for GitHub login
(`GET /api/v0/auth/github/login`, `GET /api/v0/auth/github/callback`)
(Refs `eshu-hq/eshu#8872`). The API handler keeps the HTTP shape below
and calls this port; the core-owned Service implementation satisfies it.
`eshu-api` never takes a direct table handle.

## Version

`GitHubLoginAdmissionContractVersion = "v0"`, with the port defined as:

```go
type GitHubLoginAdmission = GitHubLoginService
```

The line is vendored verbatim in
`contracts/upstream/github-login-wire-shapes.txt`, with the surrounding
contract comment, the `GitHubLoginService` port (2 methods:
`StartGitHubLogin`, `CompleteGitHubLogin`), the start/complete
request/response shapes mirroring OIDC, and the route registration. The
frozen method set is `frozenGitHubLoginAdmission` in
`contracts/upstream/github_login_admission_contract_test.go`.

Out of contract by design: the optional `GitHubProviderLister` is not
part of v0 — a dest client implementing only the 2 admission methods
silently drops runtime provider discovery and must implement the lister
or accept DB-row-only discovery. Browser-session issuance on callback
belongs to the BrowserSessions family's own seam, not this contract.

## HTTP shape

The operator-facing login flow, vendored
verbatim in `contracts/upstream/github-login-wire-shapes.txt`:

- Login: the start request selects one configured GitHub provider and
  the tenant/workspace boundary; the response carries the provider
  authorization redirect URL.
- Callback: the state and authorization code resolve into an Eshu auth
  context with the provider subject id, hashed group claims, and proof
  timestamp, mirroring OIDC.
- Failure modes fail closed without leaking provider or claim details:
  503 when the service is unconfigured (nil service unwraps to a nil
  port, never a value adapter wrapping nil).
- GitHub login requires Postgres; disabled configs return no handler.

## Enforcement

`TestGitHubLoginAdmissionContractIsVersioned` pins the version and the
frozen method set; the wiring identity test pins the exact injected
port. A renamed, added, removed, or re-signed method fails the frozen
test until the list is deliberately updated; bump
`GitHubLoginAdmissionContractVersion` alongside (manually — the bump is
convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.
