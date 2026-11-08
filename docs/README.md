# eshu-api docs

Welcome in. `eshu-api` is Eshu's HTTP front door: it checks who's
calling, works out where the request goes, and hands it to the Eshu
core over HTTP. If you're new here, start with
[Getting started](getting-started/README.md) and come back for the
rest as you need it.

## Start here

| Page | Read it when… |
| --- | --- |
| [Getting started](getting-started/README.md) | You want it running and your first call answered |
| [Guides](guides/README.md) | You need step-by-step help to operate it, set up auth, or fix a failure |
| [Concepts](concepts/README.md) | You want the mental model: what we own, what we never touch, how a request travels |
| [Reference](reference/README.md) | You need an exact flag, env var, or route classified |
| [Architecture](architecture/README.md) | You want our place in Eshu, the components, and the stack |
| [Diagrams](diagrams/README.md) | You think in pictures (every diagram lives here, with a text version) |
| [Decisions](decisions/README.md) | You want the recorded architecture decisions (ADRs) |
| [Contributing](contributing/README.md) | You're changing code and want setup, `make`, and PR rules |

## Standing references in this folder

These pages predate the tree above and stay where they are:

- [What this build serves](skeleton.md) — scope and limits of the current build.
- [Telemetry](telemetry.md) — metrics on `/metrics` and the fields in each log line.
- [Release](release.md) — tags, images, and how to check a published digest.
- [Core read contracts](../contracts/core-read-contracts.md) — the versioned read contracts dispatch relies on.

## Maintainer history

Extraction records, kept for provenance rather than day-to-day reading:

- [Parity](parity.md) and [Provenance](provenance.md) — what moved where, with source pins.
- Latency evidence per lane: `d3-latency.md`, `d4-latency.md`,
  `d6-latency.md`, `d7-latency.md`, `d8-latency.md`, `d9-latency.md`,
  `leafb-latency.md`, and `setup-latency.md` in this folder.

New here? The repo [README](../README.md) has the one-screen version.
