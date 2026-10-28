# Contributing to eshu-api

Seed state: this repo holds versioned contracts plus parity evidence
(`contracts/`, `docs/provenance.md`) and has no `go.mod` yet. The first Go
PR activates the dormant Go lanes with no workflow changes.

## CI lanes

`ci` runs path-selected lanes behind stable job names; `ci-complete` is the
only context a `main` ruleset requires. It passes when needed lanes passed
and fails when any needed lane failed — legitimately skipped lanes are fine,
so docs-only PRs (`README.md`, other `**/*.md`, `LICENSE`) run no lane.

| PR changes | Lanes that run |
| --- | --- |
| Docs only | no lane, `ci-complete` succeeds |
| Go sources (`**/*.go` outside `contracts/upstream/**`, `go.mod`, `go.sum`, `.golangci.yml`) | `lint`, `test`, `race` (+ coverage artifact) |
| Fixtures/config tests read (`testdata/**`, `**/*.json`, `**/*.yaml`/`yml`, `**/*.example.*`) | `test` only |
| Shell scripts (`**/*.sh`) | `shell` only |
| `contracts/upstream/**`, `scripts/check-parity.sh`, `docs/provenance.md` | `parity` |
| `.github/workflows/**` | `actionlint`, plus `lint`/`test`/`race`/`parity` to prove the lanes still work |

Anything unclassified runs the full Go lanes. `contracts/upstream/**` holds
byte-exact source copies (parity evidence, not build inputs) and never
triggers Go lanes on its own.

## Local proof

`make ci` runs the `parity`, `lint`, `test`, and `race` commands. Until
`go.mod` lands the Go targets notice that and pass vacuously; `parity`
(`bash scripts/check-parity.sh`) is the lane with teeth.

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
  `COVERAGE_FLOOR`, which may only rise toward the 90% finish line.
- Fork PRs run the required lanes on a read-only token with no secrets.

Deferred until their prerequisites land (tracked under `#1`): the `image`
lane (needs a `Dockerfile`), Dependabot (needs `go.mod`), the coverage
ratchet (baseline `NOT_MEASURED` until the first Go package lands).
