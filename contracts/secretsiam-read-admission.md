# SecretsIAM read admission contract

Versioned admission-contract port for the secrets/IAM reads (the five
`GET /api/v0/secrets-iam/*` routes) (Refs `eshu-hq/eshu#9669`,
satellite `eshu-hq/eshu-api#100`, core `eshu-hq/eshu#10537`).
Canonical reads persist through explicit core-owned admission
contracts: `eshu-api` keeps the HTTP shape and calls this versioned
port, which the core-owned Postgres stores satisfy jointly, composed
by the consumer. `eshu-api` never takes a direct store handle.

This is the deliberate pin the W8a lane consumes: the pin
re-declares the five mirrored signatures in this secrets/IAM-scoped
shape instead of embedding or aliasing
`IAMIdentityTrustChainStore`,
`IAMPrivilegePostureObservationStore`, `IAMSecretAccessPathStore`,
`IAMPostureGapStore`, or `IAMPostureSummaryStore`, so no other family
can cite it as coverage.

## Version

`SecretsIAMReadAdmissionContractVersion = "v0"`, with the port defined as:

```go
type SecretsIAMReadAdmission interface {
	ListSecretsIAMIdentityTrustChains(context.Context, IAMIdentityTrustChainFilter) ([]IAMIdentityTrustChainRow, error)
	ListSecretsIAMPrivilegePostureObservations(context.Context, IAMPrivilegePostureObservationFilter) ([]IAMPrivilegePostureObservationRow, error)
	ListSecretsIAMSecretAccessPaths(context.Context, IAMSecretAccessPathFilter) ([]IAMSecretAccessPathRow, error)
	ListSecretsIAMPostureGaps(context.Context, IAMPostureGapFilter) ([]IAMPostureGapRow, error)
	SummarizeSecretsIAMPosture(context.Context, string) (IAMPostureSummary, error)
}
```

The version line, the interface, the one-directional compile-time
mirror assertion, and the surrounding contract comment (narrow by
construction, explicitly OUT, rollback) are vendored verbatim in
`contracts/upstream/secretsiam-read-wire-shapes.txt`. The frozen method
set is `frozenSecretsIAMReadAdmission` in
`contracts/upstream/secretsiam_read_admission_contract_test.go`
(5 methods). The excerpt stops at the mirror assertion: the
composition-only satisfier (`secretsIAMReadAdmission`) and its
`NewReadAdmissionWithReadStore` constructor are minted core-side but
unconsumed in W8a — the lane terminates through the forward
transport only (see `contracts/w8a-termination.md`), and direct-store
serving needs a separate deliberate proof before any lane cites it.

## Read surface

The seam is read-only on purpose: every declared method is a
bounded read and the port carries no mutation surface, so a dest
consumer implementing only this contract can serve secrets/IAM reads
but cannot acquire a write it did not bargain for through it.

## In scope

- The four bounded list reads (`identity-trust-chains`,
  `privilege-posture-observations`, `secret-access-paths`,
  `posture-gaps`) admit through their re-declared store lists.
- `posture-summary` admits through `SummarizeSecretsIAMPosture`;
  the S3 external-principal grant section blends core-side on the
  forwarded call from the handler's own `GrantPosture` handle.

Explicitly OUT of this pin, recorded not silently assumed (each
keeps its own standing until a deliberate pin covers it):

- The S3 external-principal grant-posture leg
  (`GraphIAMGrantPostureStore.SummarizeS3ExternalPrincipalGrantPosture`,
  the canonical `GRANTS_ACCESS_TO` graph truth blended into the
  summary response by the handler) is not pinned here: there is no
  grant method in the frozen set, so a dest consumer cannot acquire
  graph reads through it. The handler keeps blending the graph
  section from its own `GrantPosture` handle, which stays `nil`-safe
  (the section omits instead of failing the rollup).
- The grant checks, the fixed failure messages, and the capability
  profile are handler behavior, not port shape: there is no method
  set to freeze.

## Relay posture

The secrets/IAM legs terminate in the W8a lane (see
`contracts/w8a-termination.md`): shared-key callers terminate on the
native lane ahead of the generic proxy with byte parity to the
relay, while scoped-token, OIDC-bearer, and browser-session classes
relay explicitly until D8-grade parity. The pin is recorded here
first so the termination lane consumes a minted pin, never an
unminted one.

## Enforcement

`TestSecretsIAMReadAdmissionContractIsVersioned` pins the version
and the frozen method set. A renamed, added, removed, or re-signed
method fails the frozen test until the list is deliberately updated;
bump `SecretsIAMReadAdmissionContractVersion` alongside (manually —
the bump is convention, the frozen list is the enforcement), and
this repository records the new pin before consuming it.
