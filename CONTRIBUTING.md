# Contributing to eshu-api

Skeleton state: this repo holds versioned contracts, parity evidence
(`contracts/`, `docs/provenance.md`), and its first Go increment
(`api/`, `internal/readclient/`, `cmd/eshu-api/`, `go.mod`). The Go lanes
run for real; no workflow change was needed to activate them.

## CI lanes

`ci` is a thin caller. Its jobs are `changes` (path classifier, runs on
every event), `parity` (this repo's own `scripts/check-parity.sh` lane),
`lanes` (the shared lane bodies, run from the pinned
`eshu-hq/.github` `go-collector-ci.yml` reusable at an exact commit SHA),
and `ci-complete`. The lanes inside `lanes` are `lint`, `test`, `race`,
`shell`, `image`, `govulncheck`, `gosec`, and `actionlint`; they show up as
`lanes / <lane>` in the checks list. `ci-complete` is the only context a
`main` ruleset requires. It passes when needed lanes passed and fails when
any needed lane failed — legitimately skipped lanes are fine, so docs-only
PRs (`README.md`, other `**/*.md` except `docs/provenance.md`, `LICENSE`)
run no lane. `docs/provenance.md` is a parity input (see the table below),
not a prose page. Each `run-*` gate in `ci.yml` selects its lane from the
`changes` outputs; `ci_workflow_test.go` freezes the job names, the
reusable pin, those gates, the tool pins, and the coverage floor.

| PR changes | Lanes that run |
| --- | --- |
| Docs only | no lane, `ci-complete` succeeds |
| Go sources (`**/*.go` outside `contracts/upstream/**`, `go.mod`, `go.sum`, `.golangci.yml`) | `lint`, `test`, `race` (+ coverage artifact) |
| Fixtures/config tests read (`testdata/**`, `**/*.json`, `**/*.yaml`/`yml`, `**/*.example.*`) | `test` only |
| Shell scripts (`**/*.sh`) | `shell` only |
| `contracts/upstream/**`, `scripts/check-parity.sh`, `docs/provenance.md` | `parity` |
| `.github/workflows/**` | `actionlint`, plus `lint`/`test`/`race`/`parity`/`image`/`govulncheck`/`gosec` to prove the lanes still work |
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
  in the `lint` lane): no committed counts, lists, or parity files compared against
  the tree. Today's over-cap list is empty; any future allowlist entry cites
  its child issue under `eshu-hq/eshu-api#1` and may only shrink.
- **No hidden debt**: lint runs on the whole repo, no `--new-from-rev`, no
  baselines. Exclusions in `.golangci.yml` need a reason and a linked open
  issue on the same lines.
- Coverage: the `race` lane uploads a coverprofile (14-day retention),
  posts the last 30 `go tool cover -func` lines to the job summary, and
  fails below the `coverage-floor` input in `ci.yml`, which sits at the
  measured total (past the 90% finish line), matches `COVERAGE_FLOOR` in
  the `Makefile`, and may only rise.
- Fork PRs run the required lanes on a read-only token with no secrets.

Dependabot (gomod and github-actions) is configured. The coverage ratchet
is live at the measured total and may only rise. Moving to the reusable
changed a few lane details (gosec findings ride a run artifact instead of
the Security tab, scheduled scans no longer file an issue, and scheduled
gosec findings fail the lane); the full list is in the `ci.yml` header.
