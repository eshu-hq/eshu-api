# ADR-0002: `eshu-api` owns API compute and scales horizontally

## Status

Accepted as the target architecture on 2026-11-27. The direction is
owner-accepted in the issue record below; this ADR text remains
subject to owner review in its documentation PR.

This decision supersedes [ADR-0001](0001-http-only-core-dispatch.md)
in phases. ADR-0001 remains the operative runtime boundary until the
library, security, parity, performance, and rollback gates below pass.
No storage credential may be granted to `eshu-api` merely because this
ADR exists; each credential class requires its phase gate.

The owner decision is recorded on
[eshu-api#69](https://github.com/eshu-hq/eshu-api/issues/69#issuecomment-7228023674).
It resolves the source-residence question left open by the Path A /
Path B record: API implementation source and module ownership
ultimately end in this repository. The earlier
[decline of the Path B scope-shrink](https://github.com/eshu-hq/eshu/issues/9669#issuecomment-6727342521)
also remains in force: a permanently forwarding satellite is not the
accepted end state.

## Context

The current satellite exposes the full mounted API scope, but it does
not compute API answers from storage. Routes described as terminated
still terminate in `forwardToCore`; for example
`api/d3_terminate.go` authorizes a shared-key caller and then forwards
the request on the native telemetry lane while core re-judges it as a
backstop. Other families fetch one core response and synthesize or
re-emit its shape. Some authorizer and dispatch logic executes in
`eshu-api`, but all store-backed computation—query execution,
mutation effects, sessions, audit state, PostgreSQL access, and
graph access—remains in the separate `core-api` serving process.

That arrangement made the in-tree API removable as a file tree, but it
did not make `eshu-api` the API service in the sense the migration
promised. It also created two release and deployment surfaces for one
request path, duplicated specification and posture maintenance, and
made availability and latency depend on two services in series.

The desired end state is a real, horizontally scalable API tier. It
is not a collection of route-sized microservices. `eshu-api` remains
one deployable service; ingestion, reduction/materialization,
collectors, and other background platform services remain separate
because they scale and fail on different axes.

## Decision

### 1. `eshu-api` computes the answer

A route counts as migrated only when the `eshu-api` process executes
the handler itself against PostgreSQL, Neo4j, or the content store
and makes zero HTTP calls to a core origin for that request.
Forwarding behind a local gate, including forwarding labelled
`viaNative`, is not termination or compute ownership.

Each migrated family must prove this with both:

- a route census showing no reachable `forwardToCore` path; and
- runtime evidence showing zero core-origin calls and store-query
  spans emitted by `eshu-api`.

### 2. Extract one implementation; do not fork it

Extract the existing query/serving implementation behind the
`CoreServeDeps` / `NewCoreServeMux` seam into a versioned shared
library. During transition, `eshu-api` and `core-api` consume the
same pinned library version and exactly one binary computes a given
family. Do not copy the query tree into a second hand-maintained
implementation.

After the serving cutover is proven, transfer final module/source
ownership to this repository. Contract-only packages consumed by the
CLI, MCP, reducer, or other clients remain versioned contracts; they
do not justify keeping a second HTTP serving implementation.

### 3. Make the API tier horizontally scalable

The request-serving deployment must be replica-safe from the first
compute-bearing phase:

- API replicas hold no request-local durable state. Sessions and
  other shared state live in their backing stores, not pod memory.
- PostgreSQL and Neo4j pool budgets are global arithmetic, not a
  per-pod guess. The chart must pin per-replica pool limits and leave
  primary/replica headroom after multiplying by maximum replicas.
- Read traffic may use PostgreSQL and Neo4j read replicas only where
  the operation's consistency contract permits it. Mutations,
  transactions, locks, idempotency claims, audit appends, and
  read-your-writes flows use the primary or an explicit causal
  guarantee.
- Autoscaling uses request concurrency and latency as well as CPU.
  The deployment requires bounded queues/backpressure, request
  timeouts, a PodDisruptionBudget, `preStop` draining, and a
  termination grace period longer than the server shutdown budget.
- Singleton or headless duties do not silently multiply with API
  replicas. Bootstrap seeding becomes an advisory-locked one-shot
  job. Commit/OIDC refresh duties require an explicitly
  replica-safe, leader-elected, or separate-worker disposition.

Read replicas scale query execution; API replicas scale HTTP,
authentication, and serialization. Neither removes the primary
write ceiling, hot-row/hot-node contention, poor query plans, or
replication lag. Write families therefore require their own
concurrency and contention proof before migration.

### 4. Migrate by store and risk, with hard gates

The migration proceeds in this order:

1. **P0 — freeze and measure.** Rebuild the 279-operation census by
   family, store, credential class, and read/mutation class. Draft
   the detailed security design, reconcile deployment secret hygiene,
   and pin connection-pool budgets.
2. **P1 — library proof.** An external consumer imports the extracted
   library and serves `/readyz` plus one read operation against
   fixture stores. Failure here stops the shared-library strategy
   before credentials or traffic move.
3. **P2–P4 — reads.** Migrate PostgreSQL reads, graph reads, then
   content/code reads. Shadow first; require contract parity,
   CLI/MCP consumer parity for each consumed family, and named
   latency/pool baselines at production scale for each class.
4. **P5 — sessions and authentication.** Move the full auth matrix
   atomically, including denial non-disclosure. The matrix covers
   shared-key credentials, scoped/PostgreSQL identity tokens, OIDC
   bearer [REDACTED], browser-session cookies, and credential-less
   or public-route behavior, plus the OIDC, GitHub, and SAML sign-in
   flows that create those sessions or identities.
5. **P6 — mutations and audit.** Move idempotency, audit ordering,
   and governed writes only with parallel-replica contention proof.
6. **P7 — duties and drain.** Complete headless-duty disposition,
   autoscaling, disruption, and graceful-drain behavior.
7. **P8 — cutover.** Flip the public service to compute-bearing
   `eshu-api` pods. Prove the CLI and MCP surfaces against the
   `eshu-api` endpoint for every consumed family. Scale `core-api`
   HTTP serving to zero only after every route, duty, parity,
   performance, auth, consumer, and rollback gate is green. Keep its
   last image deployable for one full release cycle.

Credentials expand only as phases require them: read-only store
roles first, then graph read access, then session/identity secrets,
and writable audit/session authority last. Secrets enter pods only
through secret references, with rotation and audit requirements in
the phase design.

## Consequences

### What gets easier

- One deployed service truthfully owns the public API contract and
  the computation behind it.
- API capacity can scale independently from ingestion and
  materialization workers.
- The extra API-to-core HTTP hop and duplicated specification,
  posture, and release maintenance disappear at cutover.
- Route ownership becomes binary and testable by default: a family
  either computes in `eshu-api` or still forwards; it cannot do both.
  An explicitly owner-granted carve-out is the only exception and
  must be named in the route census rather than inferred.

### What gets harder

- The internet-facing pod becomes a storage and identity credential
  holder. ADR-0001's narrow audit/security benefit is deliberately
  surrendered and must be replaced with least-privilege credentials,
  replica-safe secret handling, and a larger security review surface.
- API scaling can now overload shared stores if pool budgets,
  replica routing, or expensive-query controls are wrong.
- Library extraction creates a public module/API commitment during
  transition. Version skew between the library, schema migrations,
  and graph projections must be explicitly managed until ownership
  transfer completes.
- Core HTTP retirement is blocked until parity, the full auth matrix
  defined in P5, performance, concurrency, headless-duty, MCP/CLI,
  and rollback gates all pass. A route count alone is never
  sufficient evidence.

### Rejected alternatives

- **Accept the relay as the end state.** Rejected by the owner and
  inconsistent with calling `eshu-api` the API service.
- **Copy the query tree into this repo.** Rejected because it forks a
  fast-changing implementation and creates two truths.
- **Keep source and release ownership permanently in the monorepo
  while only renaming its binary to `eshu-api`.** Rejected as the
  final state because the owner selected source/module ownership in
  this repository; it remains only a possible transitional vehicle.
- **Split the API into route-sized microservices.** Rejected because
  it multiplies deployment and consistency costs without addressing
  the actual storage and write-contention limits.
