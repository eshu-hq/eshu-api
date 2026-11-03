# SAML login admission contract

Versioned admission-contract port for SAML login
(`GET /api/v0/auth/saml/providers/{provider_id}/metadata`,
`GET /api/v0/auth/saml/providers/{provider_id}/login`,
`POST /api/v0/auth/saml/providers/{provider_id}/acs`) (Refs
`eshu-hq/eshu#8872`). The API handler keeps the HTTP shape below and
calls this port; the core-owned Postgres implementation satisfies it.
`eshu-api` never takes a direct table handle.

## Version

`SAMLLoginAdmissionContractVersion = "v0"`, with the port defined as:

```go
type SAMLLoginAdmission = SAMLStore
```

The line is vendored verbatim in
`contracts/upstream/saml-login-wire-shapes.txt`, with the surrounding
contract comment, the `SAMLStore` port (5 methods:
`GetSAMLProvider`, `CreateSAMLRequest`, `ConsumeSAMLRequest`,
`ReserveSAMLReplay`, `ResolveSAMLPrincipal`), and the route
registration. The frozen method set is `frozenSAMLLoginAdmission` in
`contracts/upstream/saml_login_admission_contract_test.go`.

Out of contract by design: the optional `SAMLProviderIDLister` is not
part of v0 — a dest client implementing only the 5 aliased methods
silently drops runtime provider discovery (`RegisteredProviderIDs`
returns nil). Sessions, the assertion verifier, and the sign-in policy
reader stay wired alongside the store; session issuance belongs to the
BrowserSessions family's own seam, not this contract.

## HTTP shape

The operator-facing login flow, vendored
verbatim in `contracts/upstream/saml-login-wire-shapes.txt`:

- Metadata: the stored service-provider metadata renders as
  `samlmetadata+xml`; an unconfigured store answers 503.
- Login: the stored provider config resolves the request into a signed
  AuthnRequest redirect; a missing provider answers 404.
- ACS: the callback consumes the pending request exactly once
  (hash-matched, expiry-checked), reserves the assertion against
  replay, and resolves the principal into an Eshu auth context.
- Every ACS outcome emits a governance audit event, allowed or denied.
- A nil store fails closed with 503 on every route.

## Enforcement

`TestSAMLLoginAdmissionContractIsVersioned` pins the version and the
frozen method set. A renamed, added, removed, or re-signed method fails
the frozen test until the list is deliberately updated; bump
`SAMLLoginAdmissionContractVersion` alongside (manually — the bump is
convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.
