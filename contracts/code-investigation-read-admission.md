# CodeContentInvestigationReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. W6c 2 core pin, per the arbiter supplemental
ruling on eshu-hq/eshu-api#98 (§2B, APPROVED for exactly the W6c 2:
`POST /api/v0/code/structure/inventory`,
`POST /api/v0/code/topics/investigate`). Minted core-side at
`81b756fd84` (merge `eshu-hq/eshu#10688`); the satellite records this
pin here. A satellite PR consuming an unminted pin is invalid.
Arbiter review of this pin precedes satellite termination of the W6c
investigation legs.

## Shape

Version: `CodeContentInvestigationReadAdmissionContractVersion = "v0"`
(go/internal/query/codequery/code_content_investigation_read_admission.go).

Ceiling 3 re-declared (never embedded) methods — the two
structural-inventory reads plus the one topic investigation —
sealed, with every graph method OUT by construction:

| # | Method | Mirrors | Serves |
|---|--------|---------|--------|
| 1 | `InspectStructuralInventory(context.Context, StructuralInventoryRequest) ([]EntityContent, error)` | staying content store | structure/inventory default-kind window hydration |
| 2 | `CountStructuralInventoryByFile(context.Context, StructuralInventoryRequest) ([]StructuralInventoryFileCount, error)` | staying content store | structure/inventory `function_count_by_file` kind |
| 3 | `InvestigateCodeTopic(context.Context, CodeTopicInvestigationRequest) ([]CodeTopicEvidenceRow, error)` | staying content store | topics/investigate ranked evidence page |

There is deliberately no constructor, selector, or ReadStore surface
on this pin: the satellite satisfies this port with its own wiring;
the pin carries the read shape only. Both routes bind the shared
Postgres `codeContentGrantScope` grant.

Enforcement (all in go/internal/query/codequery):

- `code_content_investigation_read_admission_contract_test.go` —
  frozen method-set test plus version pin
  (`TestCodeContentInvestigationReadAdmissionContractIsVersioned` —
  exactly 3 methods), with the satisfiability witness and the seeded
  re-signed rejection
  (`TestCodeContentInvestigationReadAdmissionRejectsResignedMethod`).
- `code_content_investigation_read_admission_wire_shapes_test.go` —
  (`TestCodeContentInvestigationReadAdmissionWireShapesAreSatelliteConsumable`)
  proves a satellite can consume the pin using only the pin plus the
  staying shapes, and the topic read propagates the
  deadline-partial marker alongside its bounded partial rows — the
  semantics the satellite port must preserve.

## Census (per route per method)

| Method | `POST /api/v0/code/structure/inventory` | `POST /api/v0/code/topics/investigate` |
|---|---|---|
| `InspectStructuralInventory` | default-kind window hydration | — (OUT by construction) |
| `CountStructuralInventoryByFile` | `function_count_by_file` kind only | — (OUT by construction) |
| `InvestigateCodeTopic` | — (OUT by construction) | ranked evidence page, incl. the deadline-partial shape |

Reachability notes, recorded not silently assumed: the empty grant
is a blocked page built, not zero-valued — the inventory branch
answers an allocated empty `results` slice (a grantless caller is
indistinguishable from an empty index); the topic
`ErrCodeTopicDeadlineTruncated` deadline carries its bounded partial
rows alongside the marker error with `deadline_truncated` set, while
every other failure drops the rows.

## Explicitly OUT (not widened by this pin)

- Secrets/investigate (isolated least-privilege pin, §2C):
  redacted secret-line text is never bundled here.
- `Run`, `RunSingle`, and every other graph method: this pin is
  Postgres-content only.
- Every other content read (search, list, file, entity, secrets): a
  new content method is adopted only by deliberate re-declaration
  plus a frozen-list update.
- The grant checks (`codeContentGrantScope` plus the repository
  selector), the fixed failure messages, and `QueryProfile` are
  handler behavior, not port shape.
- No constructor, selector, or ReadStore surface.

## Satellite consumption

Depend on the shape, not a constructor: serve both routes' reads
through the guarded Postgres content reader the way `cmd/api`
wiring builds it today, preserving the empty-grant blocked page and
the deadline-partial semantics exactly. A satellite PR consuming
this pin for any other backend pairing is invalid without arbiter
review.
