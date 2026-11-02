# Admin identity admission contract

Versioned admission-contract port for tenant-scoped admin identity
mutations (invitation revoke, role grant/revoke, IdP group mapping
create/delete under `/api/v0/auth/admin/...`) (Refs
`eshu-hq/eshu#8872`). The API handler keeps the HTTP shape below and
calls this port; the core-owned Postgres store satisfies it. Canonical
mutations persist through explicit core admission; `eshu-api` never takes
a direct table handle.

## Version

`AdminIdentityAdmissionContractVersion = "v0"`, with the port defined as:

```go
type AdminIdentityAdmission = MutationStore
```

Both lines are vendored verbatim in
`contracts/upstream/admin-identity-wire-shapes.txt`, with the surrounding
contract comment and the `MutationStore` port. The frozen method set is
`frozenAdminIdentityAdmission` in
`contracts/upstream/admin_identity_admission_contract_test.go`
(5 methods: `RevokeAdminInvitation`, `GrantAdminRoleAssignment`,
`RevokeAdminRoleAssignment`, `CreateAdminIdPGroupMapping`,
`DeleteAdminIdPGroupMapping`).

## HTTP shape

The operator-facing mutation bodies and results, vendored
verbatim in `contracts/upstream/admin-identity-wire-shapes.txt`:

- Requests: invitation revoke, role grant/revoke, IdP mapping
  create/delete — every method scoped strictly to the caller's tenant
  (and workspace where applicable). Writes are idempotent under retry
  via active-row conflict keys.
- Response: HTTP 200 with the per-method result, including the opaque
  `mapping_ref` on mapping create.
- Auth: all-scope admin authentication with an admin tenant scope;
  otherwise 403.
- Unconfigured handler (nil or typed-nil store unwrapped to nil): 503.

## Enforcement

`TestAdminIdentityAdmissionContractIsVersioned` pins the version and the
frozen method set; the wiring identity test pins the exact injected
port. A breaking change to the mutation shape fails the frozen test
until the list is deliberately updated; bump
`AdminIdentityAdmissionContractVersion` alongside (manually — the bump is
convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.
