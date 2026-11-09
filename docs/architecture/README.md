# Architecture: eshu-api

## Place in Eshu

`eshu-api` is the HTTP front door. Callers (browser, `eshu-cli`, or
your app) talk to this repo first; this repo talks to the Eshu core
over versioned HTTP contracts; the core alone holds the graph and the
durable stores. The repo [README](../../README.md) keeps the one-screen
version, and [Diagrams](../diagrams/README.md) draws the topology.

Sibling repos: `eshu` holds the core, `eshu-cli` is the command line,
`eshu-mcp` serves tools to AI assistants, and
[eshu-deploy](https://github.com/eshu-hq/eshu-deploy) runs the full
stack. For scope and limits of the current build, see
[What this build serves](../skeleton.md).

## Components

| Component | Job |
| --- | --- |
| `cmd/eshu-api` | The binary. Parses `--addr` and `--core-base-url`, reads `ESHU_API_KEY` from env, serves. |
| `api/` | The HTTP surface: router chain (deprecation, v1 rewrite, auth gate), native lanes, relay dispatch, telemetry registry. `api/router.go` is where routes register — and where the public-route carve-outs are documented. |
| `internal/readclient/` | The versioned HTTP read-contract client seam to the core. |
| `contracts/` | The versioned read contracts dispatch relies on, starting with [Core read contracts](../../contracts/core-read-contracts.md). `contracts/upstream/` holds byte-exact parity copies, never build inputs. |
| `openapi/` | The served OpenAPI document, built from compiled constants. |

Request flow through these parts is told in
[Request story](../concepts/request-story.md); ownership lines are in
[Boundaries](../concepts/boundaries.md).

## Stack

- Go 1.26.10 (see `go.mod`), module `github.com/eshu-hq/eshu-api`.
- Image: multi-arch (`linux/amd64,linux/arm64`) build onto a distroless
  non-root base with no shell; published as `ghcr.io/eshu-hq/eshu-api`
  with SLSA provenance and an SPDX SBOM — see [Release](../release.md).
- Observability: Prometheus text on `GET /metrics` plus one structured
  JSON log line per dispatch — see [Telemetry](../telemetry.md).

The load-bearing decision behind this shape — cross to the core only
over HTTP, hold no storage handles — is recorded in
[ADR-0001](../decisions/0001-http-only-core-dispatch.md).
