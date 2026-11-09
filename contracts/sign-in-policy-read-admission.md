# Sign-in-policy read admission contract

Versioned admission-contract port for the sign-in-policy reads (`GET
/api/v0/auth/admin/sign-in-policy`, `GET /api/v0/auth/sign-in-policy`,
`GET /api/v0/auth/providers`) (Refs `eshu-hq/eshu#9669`, O6 pin 1).
Canonical reads persist through explicit core-owned admission
contracts: `eshu-api` keeps the HTTP shape and calls this versioned
port, which the core-owned sign-in-policy store satisfies.
`eshu-api` never takes a direct store handle.

The pin re-declares the single mirrored signature in this
sign-in-policy-scoped shape instead of embedding or aliasing
`SignInPolicyReadStore`, so no other family can cite it as coverage.

## Version

`SignInPolicyReadAdmissionContractVersion = "v0"`, with the port defined as:

```go
type SignInPolicyReadAdmission interface {
	GetSignInPolicy(ctx context.Context, tenantID string) (SignInPolicy, error)
}
```

The version line, the interface, the one-directional compile-time
mirror assertion, and the surrounding contract comment (narrow by
construction, explicitly OUT, rollback) are vendored verbatim in
`contracts/upstream/sign-in-policy-read-wire-shapes.txt`. The frozen
method set is `frozenSignInPolicyReadAdmission` in
`contracts/upstream/sign_in_policy_read_admission_contract_test.go`
(1 method: `GetSignInPolicy`).

## Read surface

The seam is read-only on purpose: the declared method is a bounded
read and the port carries no mutation surface, so a dest consumer
implementing only this contract can serve the policy read but cannot
acquire a write it did not bargain for through it.

## In scope

- The admin sign-in-policy GET, the public sign-in-policy GET, and
  the providers GET admit their policy read through `GetSignInPolicy`
  (mirroring `SignInPolicyReadStore`).

Explicitly OUT of this pin: the sign-in-policy mutation
(`UpsertSignInPolicy`) stays on the root `SignInPolicyMutationStore` /
`SignInPolicyAdmission` pin, which this file does not touch; grant
checks, fail-open read conventions, and fixed failure messages are
handler behavior, not port shape.

## Relay posture

The sign-in-policy read legs stay on the generic relay until the
termination lane lands; no native route moves in this record. The pin
is recorded here first so the termination lane consumes a minted pin,
never an unminted one. It unblocks the admin sign-in-policy GET on
shared-key after record plus parity plus D0 review; the public GET
and the providers GET stay gated on the O3 posture election.

## Enforcement

`TestSignInPolicyReadAdmissionContractIsVersioned` pins the version
and the frozen method set. A renamed, added, removed, or re-signed
method fails the frozen test until the list is deliberately updated;
bump `SignInPolicyReadAdmissionContractVersion` alongside (manually —
the bump is convention, the frozen list is the enforcement), and
this repository records the new pin before consuming it.
