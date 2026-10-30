# Core dispatch (Refs `eshu-hq/eshu#8872`, part of `eshu-hq/eshu#6707`)

The destination builds, tests, and ships as its own module. It serves the
versioned routing, auth, v1 alias, and deprecation surface over health and
readiness, and dispatches the authenticated `/api/*` subtree to the
configured core origin byte-for-byte (status, headers, body). No core
implementation package is imported anywhere in this tree; no shared-table
handle crosses the boundary.

## What is here

- `go.mod`: module `github.com/eshu-hq/eshu-api`, stdlib only. No import
  of a core implementation package exists anywhere in this tree.
- `api/`: the HTTP surface. `version.go` pins `CoreReadContractsVersion`
  and `SuppressionAdmissionContractVersion` to `v0` (lockstep with the
  vendored pins under `contracts/`); `v1alias.go` and `deprecation.go`
  port the in-tree middleware behavior (`/api/v1/` rewrite ahead of auth,
  `Deprecation`/`Sunset` on `/api/v0/*` only, v1 never carries them);
  `auth.go` gates `/api/*` behind the shared-key bearer credential with
  the live 401 wire shape (envelope-aware, bare `Bearer` challenge,
  correlation-ID echo-or-mint); `router.go` composes the production chain
  (deprecation outer, v1 rewrite, auth gate) over `GET /health`,
  `GET /readyz`, and the auth-gated `/api/*` subtree.
- `internal/readclient/`: the versioned HTTP read-contract client seam.
  Pinned bounds are `DefaultTimeout` 30s, `DefaultResponseByteBudget`
  262144 bytes, and the `application/eshu.envelope+json` Accept header.
  The Bearer [REDACTED] travels only in the Authorization header and is never
  logged. Routes hang under `/api/v0/` on the configured core origin.
- `cmd/eshu-api/`: the binary. Today it serves `GET /health` (served
  contract versions, intended core origin) and `GET /readyz` (probe shape)
  through the full chain, both public. `--core-base-url` is required; the
  credential arrives only via `ESHU_API_KEY` (never a flag) and an empty
  value fails closed. Authed `/api/*` calls dispatch to `--core-base-url`
  and relay the core's answer byte-for-byte (method, path, query, headers,
  body; redirects unfollowed; hop-by-hop headers stripped). Only an
  unreachable or unconfigured core answers the local permanent
  `503 backend_unavailable` with no Retry-After.
- `Dockerfile`: multi-stage build, non-root runtime, no shell.

## Credential posture

One source only: `ESHU_API_KEY`. The value never appears in flags,
logs, health output, or error bodies. Scoped-token and OIDC resolvers are
still open (see NOT_CHECKED); the shared-key gate is the only resolver.

## Proof (local, this increment)

- `go build ./...`, `go vet ./...`, `go test ./... -count=1`, and the
  race suite pass; `golangci-lint run ./...` and `golangci-lint fmt`
  are clean.
- `bash scripts/check-parity.sh` still passes: the contract surface is
  unchanged by this increment.
- The built binary serves `/health` and `/readyz` without a credential;
  `/api/v0/*` without one 401s with the live envelope shape and the v0
  deprecation headers; `/api/v1/*` 401s without them; authed calls relay
  the stub core's status, headers, and body byte-for-byte in tests, and a
  dead core answers `503 backend_unavailable` with no Retry-After.
- Dispatch timeout (30s) is pinned equal to the read-contract client
  bound; redirects are never followed; the credential crosses to the core
  only in the Authorization header.

## NOT_CHECKED

Suppression and other admission routes beyond the relay, scoped-token/OIDC
resolvers, QA deployment from the destination artifact, parity re-proof
against the live core path, concurrency and partial-failure proof,
rehearsed rollback, owner production acceptance, and in-tree removal.
Those stay open under `eshu-hq/eshu#8872`.
