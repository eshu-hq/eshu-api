# CodeHardcodedSecretReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. W6c 1 core pin, per the arbiter supplemental
ruling on eshu-hq/eshu-api#98 (§2C, isolated least-privilege,
APPROVED for exactly the one W6c secret read: `POST
/api/v0/code/security/secrets/investigate`). Minted core-side at
`81b756fd84` (merge `eshu-hq/eshu#10688`); the satellite records this
pin here. A satellite PR consuming an unminted pin is invalid.
Arbiter review of this pin precedes satellite termination of the
secret leg.

## Shape

Version: `CodeHardcodedSecretReadAdmissionContractVersion = "v0"`
(go/internal/query/codequery/code_hardcoded_secret_read_admission.go).

Ceiling 2 re-declared (never embedded) methods — the plain
investigation plus the source-reporting twin — sealed, with every
graph method OUT by construction:

| # | Method | Mirrors | Serves |
|---|--------|---------|--------|
| 1 | `InvestigateHardcodedSecrets(context.Context, HardcodedSecretInvestigationRequest) ([]HardcodedSecretFindingRow, error)` | staying content store (plain) | secrets/investigate on a plain-only store (side-table default) |
| 2 | `InvestigateHardcodedSecretsWithSource(context.Context, HardcodedSecretInvestigationRequest) ([]HardcodedSecretFindingRow, HardcodedSecretReadSource, error)` | staying content store (source twin) | secrets/investigate preferred path (labels the legacy-scan read path) |

Both methods are IN because production satisfies both: the mirror
compiles against the production content reader for the pair, proven
at mint time. The route prefers the `WithSource` twin and tolerates
the plain-only store (side-table-source default). The read binds
the shared Postgres `codeContentGrantScope` grant.

There is deliberately no constructor, selector, or ReadStore surface
on this pin: the satellite satisfies this port with its own wiring;
the pin carries the read shape only.

Enforcement (all in go/internal/query/codequery):

- `code_hardcoded_secret_read_admission_contract_test.go` — frozen
  method-set test plus version pin
  (`TestCodeHardcodedSecretReadAdmissionContractIsVersioned` —
  exactly 2 methods), with the satisfiability witness and the seeded
  re-signed rejection
  (`TestCodeHardcodedSecretReadAdmissionRejectsResignedMethod`).
- `code_hardcoded_secret_read_admission_wire_shapes_test.go` —
  (`TestCodeHardcodedSecretReadAdmissionWireShapesAreSatelliteConsumable`)
  proves a satellite can consume the pin using only the pin plus the
  staying shapes, with the redaction marker the satellite port must
  preserve.

## Census (per route per method)

| Method | `POST /api/v0/code/security/secrets/investigate` |
|---|---|
| `InvestigateHardcodedSecrets` | fallback path on a plain-only store (side-table source default) |
| `InvestigateHardcodedSecretsWithSource` | preferred path; labels the legacy-scan read path when the secret-line index is not published ready |

Reachability notes, recorded not silently assumed: the legacy-scan
annotation rides the response, never the pin; a blocked (empty)
grant answers the empty redacted-findings page with no read; the
findings always carry the
`secret_values_replaced_with_redacted_marker` redaction — raw
secret-line text never crosses this pin.

## Explicitly OUT (not widened by this pin)

- Topic and structural-inventory investigation (sibling W6c pin,
  §2B).
- `Run`, `RunSingle`, and every other graph method: this pin is
  Postgres-content only.
- Every other content read (search, list, file, entity, topic): a
  new content method is adopted only by deliberate re-declaration
  plus a frozen-list update.
- The grant checks (`codeContentGrantScope` plus the repository
  selector), the source-label response shaping, the fixed failure
  messages, and `QueryProfile` are handler behavior, not port shape.
- No constructor, builder, selector, or ReadStore surface.

## Satellite consumption

Depend on the shape, not a constructor: serve the route's read
through the guarded Postgres content reader the way `cmd/api`
wiring builds it today, preserving the redaction and the
source-label shaping exactly. A satellite PR consuming this pin for
any other backend pairing is invalid without arbiter review.
