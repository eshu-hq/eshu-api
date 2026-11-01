# Contributing to eshu-api

Skeleton state: this repo holds versioned contracts, parity evidence
(`contracts/`, `docs/provenance.md`), and its first Go increment
(`api/`, `internal/readclient/`, `cmd/eshu-api/`, `go.mod`). The Go lanes
run for real; no workflow change was needed to activate them.

## CI lanes

`ci` runs path-selected lanes behind stable job names; `ci-complete` is the
only context a `main` ruleset requires. It passes when needed lanes passed
and fails when any needed lane failed — legitimately skipped lanes are fine,
so docs-only PRs (`README.md`, other `**/*.md` except `docs/provenance.md`,
`LICENSE`) run no lane. `docs/provenance.md` is a parity input (see the
table below), not a prose page.

| PR changes | Lanes that run |
| --- | --- |
| Docs only | no lane, `ci-complete` succeeds |
| Go sources (`**/*.go` outside `contracts/upstream/**`, `go.mod`, `go.sum`, `.golangci.yml`) | `lint`, `test`, `race` (+ coverage artifact) |
| Fixtures/config tests read (`testdata/**`, `**/*.json`, `**/*.yaml`/`yml`, `**/*.example.*`) | `test` only |
| Shell scripts (`**/*.sh`) | `shell` only |
| `contracts/upstream/**`, `scripts/check-parity.sh`, `docs/provenance.md` | `parity` |
| `.github/workflows/**` | `actionlint`, plus `lint`/`test`/`race`/`parity`/`image` to prove the lanes still work |
| `Dockerfile`, `.dockerignore` | `image` (build only, no push) |
| Push to `main`, tag `v*` | `release`: multi-arch push to `ghcr.io/eshu-hq/eshu-api` with provenance, SBOM, Trivy gate; GitHub Release on tags only (see `docs/release.md`) |

Anything unclassified runs the full Go lanes. `contracts/upstream/**` holds
byte-exact source copies (parity evidence, not build inputs) and never
triggers Go lanes on its own.

## Local proof

`make ci` runs the `parity`, `lint`, `test`, and `race` commands. Every
target runs for real now that `go.mod` has landed. The `contracts/upstream/`
tree stays out of every Go command by construction: its nested `go.mod`
marker keeps the module toolchain from loading parity evidence as build
inputs (see `docs/provenance.md`), so `go build ./...` and friends never
see it.

## Rules that do not bend

- **No static gates** besides the 500-line cap on `.go` files (`wc -l` step
  in `lint`): no committed counts, lists, or parity files compared against
  the tree. Today's over-cap list is empty; any future allowlist entry cites
  its child issue under `eshu-hq/eshu-api#1` and may only shrink.
- **No hidden debt**: lint runs on the whole repo, no `--new-from-rev`, no
  baselines. Exclusions in `.golangci.yml` need a reason and a linked open
  issue on the same lines.
- Coverage: the `race` job uploads a coverprofile (14-day retention), posts
  per-package `go tool cover -func` totals, and fails below
  `COVERAGE_FLOOR`, which sits at the measured total (past the 90% finish
  line) and may only rise.
- Fork PRs run the required lanes on a read-only token with no secrets.

Deferred until its prerequisite lands (tracked under `#1`): Dependabot
(config not added yet; unblocked by `go.mod`). The coverage ratchet is
live at the measured total and may only rise.
