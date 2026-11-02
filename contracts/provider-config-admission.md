# Provider-config admission contracts

Versioned admission-contract port for admin provider-config mutations
(`POST /api/v0/auth/admin/provider-configs`,
`POST /api/v0/auth/admin/provider-configs/{provider_config_id}`,
`.../revert`, `.../enable`, `.../disable`, plus the
`.../test-connection` probe) (Refs `eshu-hq/eshu#8872`). The API handler
keeps the HTTP shape below and calls these ports; the core-owned Postgres
implementation satisfies them. Write-only secrets stay sealed — no
response, log line, or audit event ever carries plaintext or sealed
ciphertext. `eshu-api` never takes a direct table handle.

Two contracts move together: the mutation port for the writes, and the
read port backing the enable-time login-readiness guard (issue
`eshu-hq/eshu#5604`).

## Version

`ProviderConfigMutationAdmissionContractVersion = "v0"`, with the port defined as:

```go
type ProviderConfigMutationAdmission = MutationStore
```

`ProviderConfigReadAdmissionContractVersion = "v0"`, with the port defined as:

```go
type ProviderConfigReadAdmission = ReadStore
```

Both lines are vendored verbatim in
`contracts/upstream/provider-config-wire-shapes.txt`, with the surrounding
contract comments, the `MutationStore` (5 methods: `CreateProviderConfig`,
`UpdateProviderConfig`, `RevertProviderConfig`, `EnableProviderConfig`,
`DisableProviderConfig`) and `ReadStore` (3 methods:
`GetProviderConfigDetail`, `ListProviderConfigDetails`,
`ListProviderConfigRevisions`) ports, and the route registration. The
frozen method sets are `frozenProviderConfigMutationAdmission` in
`contracts/upstream/mutation_admission_contract_test.go` and
`frozenProviderConfigReadAdmission` in
`contracts/upstream/read_admission_contract_test.go`.

Out of contract by design: `test-connection` is served by the separate,
unversioned `ConnectionTester` port (the handler's enable path still gates
on its mandatory pass, and `EnableProviderConfig` takes the tested
revision id as a compare-and-swap guard), and env-managed providers
reject edit/delete. Neither is pinned here.

## HTTP shape

The operator-facing provider bodies and results, vendored
verbatim in `contracts/upstream/provider-config-wire-shapes.txt`:

- Create/update: exactly one of the OIDC, SAML, or GitHub field groups is
  read, selected by provider kind. Every secret field travels in plaintext
  over TLS in the request and is sealed by the store; no response echoes
  it back.
- Revert: a prior revision id restores non-secret configuration.
- Enable: requires the revision id a prior `test-connection` call
  actually tested; a concurrent write that changed the active revision
  fails the enable rather than activating untested configuration.
- Disable: deactivates the provider for login resolution.
- Every attempt is governance-audited, allowed or denied.
- Unconfigured handler (nil store): 503.

## Enforcement

`TestProviderConfigMutationAdmissionContractIsVersioned` and
`TestProviderConfigReadAdmissionContractIsVersioned` pin the versions and
the frozen method sets. A renamed, added, removed, or re-signed method
fails the frozen test until the list is deliberately updated; bump the
matching `...ContractVersion` alongside (manually — the bump is
convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.
