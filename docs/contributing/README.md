# Contributing to eshu-api

Thanks for helping. This page is the friendly front porch; the binding
rules live in the root [CONTRIBUTING.md](../../CONTRIBUTING.md). If the
two ever disagree, the root file wins.

## Setup

You'll need Go 1.26.11 (see `go.mod`), `golangci-lint` v2.12.2, and
`git`. Then:

```bash
go build -o /tmp/eshu-api ./cmd/eshu-api
make ci
```

## Make targets

`make ci` runs the same checks as CI: `parity`, `lint`, `test`, and
`race` (with the coverage floor). Run it before every push. CI adds
`shell`, `image`, `govulncheck`, `gosec`, and `actionlint` lanes by
changed path — the root
[CONTRIBUTING.md](../../CONTRIBUTING.md) maps which paths run which
lanes, and `bash scripts/api-seed-matrix.sh` proves each lane can go
red and green locally.

## PR rules that don't bend

- Go files stay at or under 500 lines.
- No static gates besides that cap: no committed counts, lists, or
  parity files compared against the tree.
- No hidden debt: lint runs on the whole repo with no baseline. Any
  `.golangci.yml` exclusion needs a reason and a linked open issue on
  the same lines.
- The coverage floor (`COVERAGE_FLOOR` in the `Makefile`, mirrored in
  `ci.yml`) may only rise.
- Docs-only PRs run no lane and merge through `ci-complete` — except
  `docs/provenance.md`, which is a parity input, not prose. Treat it
  accordingly.

## Security and license

There's no `SECURITY.md` or `LICENSE` file here; source files carry an
`SPDX-License-Identifier: MIT` header. If you find a security issue,
please tell the maintainers privately rather than opening a public
issue. And never commit credentials.
