# CodeDivergenceReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. Divergence content+graph core pin, per the
arbiter SUPPLEMENTAL-PIN-OWED ruling on eshu-hq/eshu-api#125
(APPROVED-owed for exactly the divergence 3:
`POST /api/v0/code/divergence/findings`, `/divergence/investigate`,
`/divergence/report`). Minted core-side at
`d584e38` (merge of `eshu-hq/eshu#10650`); the satellite records this
pin in a follow-up. A satellite PR consuming an unminted pin is
invalid. Arbiter review of this pin precedes satellite termination of
the divergence legs.

Scope note (recorded, not silently assumed): the merged pin file
comments scope the W-wave consumer to findings ONLY
(`code_divergence_read_admission.go`: "W-wave consumer: divergence
findings ONLY"). The frozen 6-method set below is shared by all three
routes — findings, investigate, and report all read through the same
`divergenceStore` + `Run` surface, and investigate/report use strict
subsets of it (see the per-route census) — so this lane cites the pin
for all three wave rows on the shared method set with this disclosure.
Widening the pin's in-scope comment to name investigate/report needs
arbiter reconcile before the follow-up spec-port lane; box-1 APPROVED
remains the promotion gate.

## Shape

Version: `CodeDivergenceReadAdmissionContractVersion = "v0"`
(go/internal/query/codequery/code_divergence_read_admission.go).

Ceiling 6 re-declared (never embedded) methods — the five content
reads (group stats, members, members-by-entity, drifted stats,
drifted rows) plus the bounded graph traversal (`Run`) — sealed, with
`RunSingle` OUT by construction:

| # | Method | Mirrors | Serves |
|---|--------|---------|--------|
| 1 | `DivergenceGroupStats(context.Context, string, codedivergence.Kind, int) ([]codedivergence.GroupStat, error)` | `divergenceStore` | `POST /api/v0/code/divergence/findings` (phase-one stat scan); `POST /api/v0/code/divergence/report` (per-kind stat scan) |
| 2 | `DivergenceMembers(context.Context, string, codedivergence.Kind, []string) (map[string][]codedivergence.Member, error)` | `divergenceStore` | all three routes (member hydration: findings window hydration; investigate point lookup; report per-kind hydration) |
| 3 | `DivergenceMembersByEntityID(context.Context, string, []string) (map[string]codedivergence.Member, error)` | `divergenceStore` | all three routes (wrapper-bypass + convention-outlier track hydration behind `Run`) |
| 4 | `DriftedFindingStats(context.Context, string) ([]codedivergence.GroupStat, error)` | `divergenceStore` | `POST /api/v0/code/divergence/findings` (drifted stat stream); `POST /api/v0/code/divergence/report` (drifted counts) |
| 5 | `DriftedFindingRows(context.Context, string, []string) (map[string]codedivergence.DriftedRow, error)` | `divergenceStore` | all three routes (drifted fact-row hydration) |
| 6 | `Run(context.Context, string, map[string]any) ([]map[string]any, error)` | `querycontract.GraphQuery` | all three routes (wrapper-bypass one-hop evidence + convention-outlier cohort sweep behind the content reads) |

There is deliberately no `WithReadStore` constructor on this pin:
the divergence seam spans the concurrent-safe graph reader plus the
guarded Postgres content reader, and minting one constructor would pick
a wiring the satellite never runs. The satellite keeps building both
backends the way `cmd/api` wiring builds them today; the pin freezes
only the read shape the handlers may call through. The compile-time
mirror assertion in the pin file
(`var _ CodeDivergenceReadAdmission = (*codeDivergenceReadAdmissionMirror)(nil)`,
embedding `divergenceStore` plus `querycontract.GraphQuery`) proves the
pair the handlers run against today satisfies the pin.

Enforcement (all in go/internal/query/codequery):

- `code_divergence_read_admission_contract_test.go` — frozen
  method-set test plus version pin
  (`TestCodeDivergenceReadAdmissionContractIsVersioned` — exactly 6
  methods in the listed order), with the satisfiability witness
  (`codeDivergenceReadAdmissionWitness` must implement the pin) and the
  seeded rejection RED half
  (`TestCodeDivergenceReadAdmissionRejectsResignedMethod` — a witness
  with a re-signed `DriftedFindingRows` MUST NOT implement the pin).

## Census (per route per method)

| Method | `divergence/findings` | `divergence/investigate` | `divergence/report` |
|---|---|---|---|
| `DivergenceGroupStats` | phase-one stat scan (all kinds; exact nominates wrapper-only pages) | — (point lookup; OUT by construction) | per-kind stat scan |
| `DivergenceMembers` | window hydration (exact/renamed/drifted-by-row) | point lookup (exact/renamed families) | per-kind window hydration |
| `DivergenceMembersByEntityID` | wrapper + outlier track hydration | wrapper-target + outlier-cohort hydration | wrapper + outlier track hydration |
| `DriftedFindingStats` | drifted stat stream | — (point lookup; OUT by construction) | drifted counts |
| `DriftedFindingRows` | drifted window hydration | drifted point lookup | drifted hydration |
| `Run` | wrapper one-hop evidence + outlier cohort sweep | wrapper-target qualification + outlier-cohort selection | wrapper qualification (200-family cap) + whole outlier sweep |

Reachability notes, recorded not silently assumed: the investigate leg
runs no stat scan by construction (it addresses one finding); the
report leg caps wrapper qualification at 200 families and hydration at
500 groups per kind with per-kind truncation flags; the outlier track
pages after the stat stream on findings tails; a graph track the
bounded read budget cuts short degrades to a counted
`*_graph_timeout` suppression with its kind truncated instead of
failing the call; `RunSingle` is OUT everywhere by construction.

Every other `GraphQuery` and content-store shape is OUT: `RunSingle`,
point and list reads, file-content legs, `Search*`/`List*` legs,
ledger/summary/snapshot surface, and every other `Get*` leg — other
families' standing, unchanged.

## Explicitly OUT (not widened by this pin)

- The single-row graph read (`RunSingle`) stays unpinned here: the
  divergence lane never issues one.
- The search legs and the file-content point reads keep the CodeSearch,
  Content search, and D13 point pins; there is no search or file method
  in the frozen set.
- The list legs stay unpinned here for the same reason.
- Grant checks (`codeGrantAccessFilter`, `codeContentGrantScope`),
  the capability short-circuit, the repository selector, the fixed
  failure messages, suppression assembly, and `QueryProfile` (handler
  behavior, not port shape: there is no method set to freeze).
- No constructor, builder, selector, or ReadStore surface: the
  satellite satisfies this port with its own wiring; the pin carries
  the read shape only.

## Satellite consumption

Depend on the shape, not a constructor: serve the three routes' reads
through the existing graph reader (satisfying `Run`) plus the guarded
Postgres content reader (satisfying the five content reads), the
same pair `cmd/api` wiring builds today. A satellite PR consuming this
pin for any other backend pair is invalid without arbiter review.
