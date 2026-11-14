# CodeDivergenceReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. Divergence 3 core pin, per the arbiter
supplemental ruling on eshu-hq/eshu-api#125 (SUPPLEMENTAL-PIN-OWED,
APPROVED for exactly the divergence 3: `POST
/api/v0/code/divergence/findings`,
`/divergence/investigate`, `/report`). Minted core-side at
`d584e38` (merge of `eshu-hq/eshu#10650`); the satellite records this
pin in a follow-up. A satellite PR consuming an unminted pin is
invalid. Arbiter review of this pin precedes satellite termination
of the divergence legs.

## Shape

Version: `CodeDivergenceReadAdmissionContractVersion = "v0"`
(go/internal/query/codequery/code_divergence_read_admission.go).

Ceiling 6 re-declared (never embedded) methods — the five staying
content reads (`DivergenceGroupStats`, `DivergenceMembers`,
`DivergenceMembersByEntityID`, `DriftedFindingStats`,
`DriftedFindingRows`) plus the bounded graph traversal (`Run`) —
sealed, with `RunSingle` and every other `GraphQuery` leg OUT by
construction:

| # | Method | Mirrors | Serves |
|---|---|--------|--------|
| 1 | `DivergenceGroupStats(context.Context, string, codedivergence.Kind, int) ([]codedivergence.GroupStat, error)` | staying `divergenceStore` | `POST /api/v0/code/divergence/findings` (phase-one rank over every selected kind); `POST /api/v0/code/divergence/report` (per-kind scan windows, wrapper nominators) |
| 2 | `DivergenceMembers(context.Context, string, codedivergence.Kind, []string) (map[string][]codedivergence.Member, error)` | staying `divergenceStore` | `POST /api/v0/code/divergence/findings` (window hydration incl. wrapper nominations through the exact column); `POST /api/v0/code/divergence/investigate` (exact/renamed point lookup); `POST /api/v0/code/divergence/report` (content-kind windows plus wrapper nominations) |
| 3 | `DivergenceMembersByEntityID(context.Context, string, []string) (map[string]codedivergence.Member, error)` | staying `divergenceStore` | `POST /api/v0/code/divergence/findings` (wrapper qualification and outlier cohort hydration); `POST /api/v0/code/divergence/investigate` (wrapper-target and outlier-cohort point lookups); `POST /api/v0/code/divergence/report` (both graph tracks) |
| 4 | `DriftedFindingStats(context.Context, string) ([]codedivergence.GroupStat, error)` | staying `divergenceStore` | `POST /api/v0/code/divergence/findings` (drifted stat stream); `POST /api/v0/code/divergence/report` (drifted counts) |
| 5 | `DriftedFindingRows(context.Context, string, []string) (map[string]codedivergence.DriftedRow, error)` | staying `divergenceStore` | `POST /api/v0/code/divergence/findings` (drifted window hydration); `POST /api/v0/code/divergence/investigate` (drifted point lookup); `POST /api/v0/code/divergence/report` (drifted content kind) |
| 6 | `Run(context.Context, string, map[string]any) ([]map[string]any, error)` | `querycontract.GraphQuery` | `POST /api/v0/code/divergence/findings` (wrapper-bypass one-hop qualification and convention-outlier cohort sweep); `POST /api/v0/code/divergence/investigate` (both graph tracks behind the point lookup); `POST /api/v0/code/divergence/report` (both graph tracks behind the rollup, degrading to counted suppressions on timeout) |

There is deliberately no `WithReadStore` constructor on this pin:
the divergence seam spans the guarded Postgres content reader plus
the concurrent-safe graph reader, and minting one constructor would
pick a wiring the satellite never runs. The satellite keeps building
both backends the way `cmd/api` wiring builds them today; the pin
freezes only the read shape the handlers may call through. The
compile-time witness assertion in the pin file
(`var _ CodeDivergenceReadAdmission = (*codeDivergenceReadAdmissionMirror)(nil)`,
embedding the staying `divergenceStore` plus
`querycontract.GraphQuery`) proves the pair the handlers run against
today satisfies the pin.

Enforcement (all in go/internal/query/codequery):

- `code_divergence_read_admission_contract_test.go` — frozen
  method-set test plus version pin
  (`TestCodeDivergenceReadAdmissionContractIsVersioned` — exactly 6
  methods: `DivergenceGroupStats`, `DivergenceMembers`,
  `DivergenceMembersByEntityID`, `DriftedFindingStats`,
  `DriftedFindingRows`, `Run`), with the satisfiability witness
  (`codeDivergenceReadAdmissionWitness` must implement the pin) and
  the seeded rejection RED half
  (`TestCodeDivergenceReadAdmissionRejectsResignedMethod` — a witness
  with a re-signed `DriftedFindingRows` MUST NOT implement the pin).

## Census (per route per method)

| Method | `findings` | `investigate` | `report` |
|---|---|---|---|
| `DivergenceGroupStats` | phase-one rank over every selected kind | — (point lookup needs no stat stream; OUT by construction) | per-kind scan windows; wrapper nominators |
| `DivergenceMembers` | window hydration, incl. wrapper nominations through the exact column | exact/renamed point lookup by fingerprint | content-kind windows plus wrapper nominations |
| `DivergenceMembersByEntityID` | wrapper qualification and outlier cohort hydration | wrapper-target and outlier-cohort point lookups | both graph tracks |
| `DriftedFindingStats` | drifted stat stream | — (point lookup reads the fact row directly; OUT by construction) | drifted counts |
| `DriftedFindingRows` | drifted window hydration | drifted point lookup by writer finding id | drifted content kind |
| `Run` | wrapper-bypass one-hop qualification and convention-outlier cohort sweep | both graph tracks behind the point lookup | both graph tracks behind the rollup (timeouts degrade to counted suppressions) |

Reachability notes, recorded not silently assumed: the investigate
leg runs no stat stream by construction (one addressed finding, not
a page); the drifted investigate leg reads the fact row directly;
the report rollup degrades a graph track the bounded read budget
cuts short to a counted `*_graph_timeout` suppression with its kind
truncated instead of failing the other four kinds; the findings page
keeps the loud deadline.

Every other `GraphQuery` and content shape is OUT: `RunSingle`,
point and list reads, file-content legs, ledger/summary/snapshot
surface, and every other `Search*`/`List*`/`Get*` leg — other
families' standing, unchanged.

## Explicitly OUT (not widened by this pin)

- The findings-only comment embedded in the pin file at the mint:
  the file says "divergence findings ONLY (per the arbiter
  supplemental ruling on eshu-hq/eshu-api#125)". The ruling quoted on
  the issue covers all 3 routes (per-route census verified for
  findings/investigate/report, both graph tracks, one pin), and the
  mint PR records that "one pin covers findings, investigate, and
  report" — the same 6 methods the frozen list pins. This lane
  consumes the pin for exactly those 3 routes and cites the ruling
  and the mint record, not the file comment, as the scope basis. A
  core follow-up clarifying the comment is owed to the core side, not
  to this lane.
- Every other `GraphQuery` method (`RunSingle` and the rest): this
  pin adopts `Run` only; a new graph method is adopted only by
  deliberate re-declaration plus a frozen-list update.
- The grant checks, the capability short-circuit, the fixed failure
  messages, and `QueryProfile` (handler behavior, not port shape:
  there is no method set to freeze).
- No constructor, builder, selector, or `ReadStore` surface: the
  satellite satisfies this port with its own wiring; this pin
  carries the read shape only.

## Satellite consumption

Depend on the shape, not a constructor: serve the three routes'
reads through the existing graph reader (satisfying `Run`) plus the
guarded Postgres content reader (satisfying the five content
reads), the same pair `cmd/api` wiring builds today. A satellite PR
consuming this pin for any other backend pair is invalid without
arbiter review.
