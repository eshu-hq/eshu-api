# Retirement admission contract

Versioned admission-contract port for the operator repository retirement
trigger at `POST /api/v0/admin/repository-retirements` (Refs
`eshu-hq/eshu#8872`). The API handler keeps the HTTP shape below and
calls this port; the core-owned Trigger satisfies it. The retire sequence
(mark, fence, purge) persists through explicit core admission; `eshu-api`
never takes a direct trigger handle.

## Version

`RetirementAdmissionContractVersion = "v0"`, with the port defined as:

```go
type RetirementAdmission = Triggerer
```

Both lines are vendored verbatim in
`contracts/upstream/retirement-wire-shapes.txt`, with the surrounding
contract comment and the `Triggerer` port. The frozen method set is
`frozenRetirementAdmission` in
`contracts/upstream/retirement_admission_contract_test.go`
(1 method: `TriggerRetirement`).

## HTTP shape

The operator-facing POST body and the trigger result, vendored
verbatim in `contracts/upstream/retirement-wire-shapes.txt`:

- Request: repository selector resolved through the catalog, retired
  through mark, fence, and purge in the core trigger.
- Response: HTTP 200 with the trigger result for the retired scope.
- Auth: operator authentication; otherwise 403.
- Unconfigured handler (nil catalog or nil/typed-nil trigger): 503.

## Enforcement

`TestRetirementAdmissionContractIsVersioned` pins the version and the
frozen method set; the wiring identity test pins the exact injected
port. A breaking change to the trigger shape fails the frozen test
until the list is deliberately updated; bump
`RetirementAdmissionContractVersion` alongside (manually — the bump is
convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.
