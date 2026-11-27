# Architecture decisions: eshu-api

Short records of the decisions that shape this repo, so future-us
doesn't have to re-derive them from code archaeology. We only write an
ADR for a decision that's already recorded somewhere authoritative —
this folder indexes decisions, it doesn't invent them.

## Index

| ADR | Decision | Status |
| --- | --- | --- |
| [ADR-0001](0001-http-only-core-dispatch.md) | Cross to the core only over HTTP; hold no storage handles | Accepted; operative until ADR-0002 phase gates supersede it |
| [ADR-0002](0002-eshu-api-owns-api-compute.md) | `eshu-api` owns API compute and scales horizontally | Accepted target; phased implementation |

## Format

One file per decision, named `NNNN-short-title.md`, with:

- **Status** — Proposed, Accepted, Superseded (say by what), or Deprecated.
- **Context** — the forces at play when the call was made.
- **Decision** — what was decided, in active voice.
- **Consequences** — what got easier, what got harder, what to watch.

Keep each ADR to the decision and its evidence. Link the code or doc
that proves it rather than restating the implementation.
