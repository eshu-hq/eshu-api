# ADR-0001: Cross to the core only over HTTP

## Status

Accepted. Recorded from [What this build serves](../skeleton.md),
which states that no core implementation package is imported anywhere
in this tree and no shared-table handle crosses the boundary.

## Context

`eshu-api` was split out of the core so the public HTTP surface —
routing, auth, version handling — could ship on its own schedule. The
tempting shortcut was to import core packages or open the same stores
directly. That would have coupled releases, widened the public
surface's access to storage and secrets, and made the split cosmetic.

## Decision

Everything crosses to the core over versioned HTTP contracts. This
repo never opens a database table, queue, or graph store, and imports
no core implementation package. The credential arrives only via
`ESHU_API_KEY` (never a flag), an empty value fails closed, and an
unreachable core answers a local `503 backend_unavailable` with no
`Retry-After` so callers can tell our failure from the core's.

## Consequences

- The two parts release independently; moving the core is one flag
  (`--core-base-url`).
- Auditing this repo means auditing routing, the gate, and dispatch —
  not a second data layer. See [Boundaries](../concepts/boundaries.md).
- The cost is a hop: dispatch latency includes the core call.
  [Telemetry](../telemetry.md) exists partly to keep that visible, and
  the per-lane latency notes in `docs/` record the evidence.
