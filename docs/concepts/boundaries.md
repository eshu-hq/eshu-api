# Boundaries: what eshu-api owns (and never touches)

Think of `eshu-api` as the front desk. Callers never talk to the core's
stores directly. They talk to us over HTTP, we check the request, and
then we either answer it ourselves or pass it on to the core and relay
the reply. Keeping the job small is the point: storage handles and
secrets stay out of the public HTTP surface, and this layer can ship
without waiting on the core. The picture is in
[Diagrams](../diagrams/README.md).

## We own

- The HTTP layer: routing, auth, `/api/v0/` and `/api/v1/` handling.
- The request and response shapes under `contracts/` (start with
  [Core read contracts](../../contracts/core-read-contracts.md)).
- Local answers: `GET /health`, `GET /readyz`, `GET /metrics`, the
  compiled `GET /api/v0/openapi.json` document, docs/redoc shells, and
  a handful of reads we now serve natively.
- Local failures: a `401` when a gated route gets no usable token,
  and the local `503 backend_unavailable` when the core is
  unreachable.

## We never touch

- A database table, queue, or graph store. We hold no storage handles
  of our own. Everything crosses to the core over HTTP, which is what
  lets the two parts release on their own schedule — that decision is
  recorded in
  [ADR-0001](../decisions/0001-http-only-core-dispatch.md).
- Trust decisions on the public pre-auth routes. Credential bodies
  cross to the core as opaque bytes; the core decides who gets in.
- The core's bytes on the way back. Most replies are the core's
  status, headers, and body exactly as sent. Redirects from the core
  aren't followed, either — you see what the core sent.

## Why the line matters

Because the boundary is HTTP, moving the core is one flag
(`--core-base-url`). And because we add no storage surface, auditing
this repo means auditing routing, the gate, and dispatch — not a
second copy of the core's data layer.
