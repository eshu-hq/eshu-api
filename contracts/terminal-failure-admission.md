# Terminal-failure admission contract

Versioned admission-contract port for the operator terminal-failure
acknowledgement mutation at
`POST /api/v0/admin/collector-failures/acknowledge` (Refs
`eshu-hq/eshu#8872`). The API handler keeps the HTTP shape below and
calls this port; the core-owned Postgres store satisfies it. Canonical
mutations persist through explicit core admission; `eshu-api` never takes
a direct table handle.

## Version

`TerminalFailureAdmissionContractVersion = "v0"`, with the port defined as:

```go
type TerminalFailureAdmission = Acknowledger
```

Both lines are vendored verbatim in
`contracts/upstream/terminal-failure-wire-shapes.txt`, with the surrounding
contract comment and the compile assertion. The frozen method set is
`frozenTerminalFailureAdmission` in
`contracts/upstream/terminal-failure_admission_contract_test.go`
(4 methods: `ResolveTarget`, `Acknowledge`, `ListTerminalFailureAcks`,
`ListTerminalFailures`).

## HTTP shape

The operator-facing POST body and the acknowledgement response, vendored
verbatim in `contracts/upstream/terminal-failure-wire-shapes.txt`:

- Request: `collector_kind`, `instance_id`, `scope_id` (all required),
  `reason` (required, at most 512 characters, must say why the failure is
  expected), `idempotency_key` (required; a refused request leaves its key
  unconsumed for a corrected retry).
- Response: HTTP 200 `acknowledged` with the idempotency key and the
  per-request result; 409 when the key was already used with a different
  target or is already in progress.
- Auth: admin (all-scopes) token; unauthenticated dev mode stays open
  and a scoped token is denied with 403.
- Unconfigured handler (nil store): 503.

## Enforcement

`TestTerminalFailureAdmissionContractIsVersioned` pins the version and the
frozen method set; the core-owned `TerminalFailureAckStore` satisfies the
port at compile time. A breaking change to the admission shape fails the
frozen test until the list is deliberately updated; bump
`TerminalFailureAdmissionContractVersion` alongside (manually — the bump
is convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.
