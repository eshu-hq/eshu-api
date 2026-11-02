# Setup wizard admission contract

Versioned admission-contract port for the first-run setup wizard
mutations at `POST /api/v0/auth/setup/claim`, `POST
/api/v0/auth/setup/admin`, and `POST /api/v0/auth/setup/mfa` (plus the
public pre-auth `GET /api/v0/auth/setup-state`) (Refs
`eshu-hq/eshu#8872`). The API handler keeps the HTTP shape below and
calls this port; the core-owned Postgres implementation satisfies it.
The wizard mutations persist through explicit core admission; `eshu-api`
never takes a direct table handle.

## Version

`SetupAdmissionContractVersion = "v0"`, with the port defined as:

```go
type SetupAdmission = SetupStore
```

Both lines are vendored verbatim in
`contracts/upstream/setup-wire-shapes.txt`, with the surrounding
contract comment, the `SetupStore` port, and the input/owner/response
shapes. The frozen method set is `frozenSetupAdmission` in
`contracts/upstream/setup_admission_contract_test.go`
(5 methods: `SetupNeeded`, `VerifyBootstrapCredential`,
`ResolveSetupOwner`, `RotateSetupPassword`, `CompleteSetupMFA`).

## HTTP shape

The operator-facing wizard bodies and results, vendored
verbatim in `contracts/upstream/setup-wire-shapes.txt`:

- Claim: submitted plaintext username/password checked against the
  sealed bootstrap credential envelope. A wrong guess answers 401 with
  no signal distinguishing missing, consumed, or mismatched
  credentials, so instance state cannot be probed.
- Admin: new administrator password (server-side 12+ character floor)
  rotated in the same store-layer transaction that revokes the prior
  credential.
- MFA: caller-hashed recovery codes rotated with the MFA factor while
  the bootstrap envelope is permanently consumed in one
  advisory-locked critical section; a concurrent winner answers 409
  for the loser rather than claiming success for unusable codes.
- Every mutating route re-checks `SetupNeeded` and fails closed with
  410 Gone once the wizard is sealed.
- Unconfigured handler (nil or typed-nil store unwrapped to nil): 503.

## Enforcement

`TestSetupAdmissionContractIsVersioned` pins the version and the
frozen method set; the wiring identity test pins the exact injected
port. A renamed, added, removed, or re-signed method fails the frozen
test until the list is deliberately updated; bump
`SetupAdmissionContractVersion` alongside (manually — the bump is
convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.
