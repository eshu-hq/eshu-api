# Sign-in policy admission contract

Versioned admission-contract port for the admin sign-in policy mutation
at `PATCH /api/v0/auth/admin/sign-in-policy` (Refs
`eshu-hq/eshu#8872`). The API handler keeps the HTTP shape below and
calls this port; the core-owned Postgres store satisfies it. Canonical
mutations persist through explicit core admission; `eshu-api` never takes
a direct table handle.

## Version

`SignInPolicyAdmissionContractVersion = "v0"`, with the port defined as:

```go
type SignInPolicyAdmission = SignInPolicyMutationStore
```

Both lines are vendored verbatim in
`contracts/upstream/sign-in-wire-shapes.txt`, with the surrounding
contract comment and the partial-update request struct. The frozen method
set is `frozenSignInPolicyAdmission` in
`contracts/upstream/sign_in_policy_admission_contract_test.go`
(1 method: `UpsertSignInPolicy`).

## HTTP shape

The operator-facing PATCH body and the policy response, vendored
verbatim in `contracts/upstream/sign-in-wire-shapes.txt`:

- Request: partial update — `RequireSSO`, `AllowLocalUserCreation`,
  `RequireMFAForAllUsers`, `IdleTimeoutSeconds`,
  `AbsoluteTimeoutSeconds`. A nil field is left unchanged.
- Guardrails (checked atomically inside the store's row-locked
  transaction): `require_sso` needs a provider config with a passing
  connection test and an admin with SSO proof; a non-zero absolute
  timeout must not be shorter than a non-zero idle timeout. A breach
  answers 400.
- Response: HTTP 200 with the stored policy detail.
- Auth: all-scope admin authentication with an admin tenant scope;
  otherwise 403.
- Unconfigured handler (nil or typed-nil store unwrapped to nil): 503.

## Enforcement

`TestSignInPolicyAdmissionContractIsVersioned` pins the version and the
frozen method set; the wiring identity test pins the exact injected
port. A breaking change to the admission shape fails the frozen test
until the list is deliberately updated; bump
`SignInPolicyAdmissionContractVersion` alongside (manually — the bump is
convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.
