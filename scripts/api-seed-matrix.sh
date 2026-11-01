#!/usr/bin/env bash
# Seeded RED/GREEN matrix for the locally provable CI lanes.
#
# Local runner, not a CI gate: for each seed it plants a defect, asserts the
# named lane command turns red AND names the seed file, removes the seed, and
# asserts the lane turns green again. It never compares the tree against a
# committed count or list.
#
# 14 checks: vet, lint, format, race, tidy, the 500-line cap, and the
# coverage floor (RED + GREEN each).
#
# Lane commands are the exact steps the pinned eshu-hq/.github go-collector-ci
# reusable runs (ci.yml calls it), not `make lint`: the Makefile lint recipe
# chains its steps with `;`, so only the last step sets the exit status and
# earlier failures (vet, golangci-lint, fmt, tidy) are swallowed. The floor
# checks use `make race`, whose last command is the floor comparison.
# The ci-complete aggregation and the govulncheck/gosec/actionlint lanes are
# proven by the reusable's own selftest and are not reseeded here.
#
# Usage: bash scripts/api-seed-matrix.sh
# Needs: go, rg (ripgrep), golangci-lint v2.12.2 (go install puts it in GOPATH/bin).
# Must leave the tree exactly as found: every seed is removed after its check.
set -euo pipefail

cd "$(dirname "$0")/.."

export PATH="$PATH:${GOBIN:-$(go env GOPATH)/bin}"

TMP="$(mktemp -d)"
LOG="$TMP/last.log"
pass=0
fail=0
had_coverage_out=0
[ -e coverage.out ] && had_coverage_out=1

# go.sum is absent while the module has no dependencies; restore what is tracked.
restore_mod() {
	git checkout -q -- go.mod
	if git ls-files --error-unmatch go.sum >/dev/null 2>&1; then
		git checkout -q -- go.sum
	fi
}

cleanup() {
	rm -f zz_seed_vet.go zz_seed_lint.go zz_seed_format.go \
		zz_seed_race_test.go zz_seed_cap.go
	restore_mod 2>/dev/null || true
	[ "$had_coverage_out" -eq 0 ] && rm -f coverage.out
	rm -rf "$TMP"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

# Refuse only on leftover seeds from an earlier run; unrelated uncommitted
# work is expected. The end-of-run check diffs the tree against a snapshot.
if ls zz_seed_* >/dev/null 2>&1; then
	echo "FAIL: leftover seed files present; refusing to seed"
	ls zz_seed_*
	exit 1
fi
if ! git diff --exit-code -- go.mod go.sum >/dev/null 2>&1; then
	echo "FAIL: go.mod/go.sum already drifted; refusing to seed"
	exit 1
fi
SNAPSHOT="$TMP/status-before.txt"
git status --porcelain >"$SNAPSHOT"

ok() { # $1: check name
	echo "PASS: $1"
	pass=$((pass + 1))
}
bad() { # $1: check name
	echo "FAIL: $1"
	tail -n 20 "$LOG" 2>/dev/null || true
	fail=$((fail + 1))
}

# Seeds live in the root package (ciworkflow), which has only a _test file.

# --- vet: seeded printf mismatch turns `go vet ./...` red ---
cat >zz_seed_vet.go <<'EOF'
package ciworkflow

import "fmt"

var _ = fmt.Sprintf("%d", "not-a-number")
EOF
if go vet ./... >"$LOG" 2>&1; then
	bad "vet RED on seeded printf mismatch (lane stayed green)"
elif rg -q "zz_seed_vet.go" "$LOG"; then
	ok "vet RED on seeded printf mismatch"
else
	bad "vet RED on seeded printf mismatch (no finding on the seed)"
fi
rm -f zz_seed_vet.go
if go vet ./... >"$LOG" 2>&1; then
	ok "vet GREEN after seed removal"
else
	bad "vet GREEN after seed removal"
fi

# --- lint: seeded strings.Replace with -1 trips staticcheck QF1004 ---
cat >zz_seed_lint.go <<'EOF'
package ciworkflow

import "strings"

var _ = strings.Replace("aaa", "a", "b", -1)
EOF
if golangci-lint run ./... >"$LOG" 2>&1; then
	bad "lint RED on seeded QF1004 (lane stayed green)"
elif rg -q "zz_seed_lint.go.*QF1004" "$LOG"; then
	ok "lint RED on seeded QF1004"
else
	bad "lint RED on seeded QF1004 (no QF1004 finding on the seed)"
fi
rm -f zz_seed_lint.go
if golangci-lint run ./... >"$LOG" 2>&1; then
	ok "lint GREEN after seed removal"
else
	bad "lint GREEN after seed removal"
fi

# --- format: seeded unformatted file trips the gofumpt diff check ---
printf 'package ciworkflow\n\nfunc zzSeedFormat() { x:=1; _=x }\n' >zz_seed_format.go
if golangci-lint fmt --diff ./... >"$LOG" 2>&1; then
	bad "format RED on seeded unformatted file (lane stayed green)"
elif rg -q "zz_seed_format.go" "$LOG"; then
	ok "format RED on seeded unformatted file"
else
	bad "format RED on seeded unformatted file (no diff on the seed)"
fi
rm -f zz_seed_format.go
if golangci-lint fmt --diff ./... >"$LOG" 2>&1; then
	ok "format GREEN after seed removal"
else
	bad "format GREEN after seed removal"
fi

# --- race: seeded data race fails the race lane command ---
cat >zz_seed_race_test.go <<'EOF'
package ciworkflow

import "testing"

func TestSeedMatrixRace(t *testing.T) {
	x := 0
	done := make(chan bool)
	go func() { x = 1; done <- true }()
	_ = x
	<-done
}
EOF
if go test ./... -race -count=1 -covermode=atomic -coverprofile="$TMP/coverage.out" >"$LOG" 2>&1; then
	bad "race RED on seeded data race (lane stayed green)"
elif rg -q "DATA RACE" "$LOG" && rg -q "zz_seed_race_test.go" "$LOG"; then
	ok "race RED on seeded data race"
else
	bad "race RED on seeded data race (no race reported on the seed)"
fi
rm -f zz_seed_race_test.go
if go test ./... -race -count=1 -covermode=atomic -coverprofile="$TMP/coverage.out" >"$LOG" 2>&1; then
	ok "race GREEN after seed removal"
else
	bad "race GREEN after seed removal"
fi

# --- tidy: seeded requirement drift trips go mod tidy -diff ---
# Lane: `go mod tidy -diff` then `git diff --exit-code -- go.mod go.sum`.
go mod edit -require=gopkg.in/yaml.v3@v3.0.0
if go mod tidy -diff >"$LOG" 2>&1; then
	bad "tidy RED on seeded drift (lane stayed green)"
elif rg -q "go\.mod" "$LOG"; then
	ok "tidy RED on seeded drift"
else
	bad "tidy RED on seeded drift (diff does not name go.mod)"
fi
restore_mod
if go mod tidy -diff >"$LOG" 2>&1 && git diff --exit-code -- go.mod go.sum >>"$LOG" 2>&1; then
	ok "tidy GREEN after seed removal"
else
	bad "tidy GREEN after seed removal"
fi

# --- 500-line cap: a 501-line .go file trips the lane's wc -l step ---
# Exact lane step from the reusable; no allowlist exists to bypass it.
cap_lane() {
	local over
	over=$(find . -name '*.go' -not -path './vendor/*' -exec wc -l {} + | awk '$1 > 500 && $2 != "total" {print}')
	if [ -n "$over" ]; then
		echo "$over"
		echo "FAIL: .go files over 500 lines"
		return 1
	fi
}
{
	echo "package ciworkflow"
	echo ""
	for i in $(seq 1 499); do echo "var _ = $i"; done
} >zz_seed_cap.go
if [ "$(wc -l <zz_seed_cap.go)" -ne 501 ]; then
	echo "FAIL: cap seed is not 501 lines"
	exit 1
fi
if cap_lane >"$LOG" 2>&1; then
	bad "cap RED on seeded 501-line file (lane stayed green)"
elif rg -q "zz_seed_cap.go" "$LOG"; then
	ok "cap RED on seeded 501-line file"
else
	bad "cap RED on seeded 501-line file (seed not named)"
fi
rm -f zz_seed_cap.go
if cap_lane >"$LOG" 2>&1; then
	ok "cap GREEN after seed removal"
else
	bad "cap GREEN after seed removal"
fi

# --- coverage floor: `make race` with an unreachable floor fails ---
# The real floor is the Makefile COVERAGE_FLOOR, which must equal the
# coverage-floor input in ci.yml (CI enforces the ci.yml number).
if make race COVERAGE_FLOOR=100 >"$LOG" 2>&1; then
	bad "floor RED at 100 (make race stayed green)"
elif rg -q "FAIL: coverage below floor" "$LOG"; then
	ok "floor RED at 100 ($(rg -o 'total=[0-9.]+%' "$LOG" | head -n 1))"
else
	bad "floor RED at 100 (no floor failure reported)"
fi
mk_floor=$(sed -n 's/^COVERAGE_FLOOR ?= \([0-9.]*\).*/\1/p' Makefile | head -n 1)
ci_floor=$(sed -n 's/^ *coverage-floor: \([0-9.]*\).*/\1/p' .github/workflows/ci.yml | head -n 1)
if [ -z "$mk_floor" ] || [ "$mk_floor" != "$ci_floor" ]; then
	echo "makefile floor='$mk_floor' ci.yml floor='$ci_floor'" >"$LOG"
	bad "floor GREEN (Makefile and ci.yml floors differ or are missing)"
elif make race >"$LOG" 2>&1 && rg -q "floor=${mk_floor//./\\.}%" "$LOG"; then
	ok "floor GREEN at $mk_floor ($(rg -o 'total=[0-9.]+%' "$LOG" | head -n 1))"
else
	bad "floor GREEN at $mk_floor"
fi

echo "seed matrix: pass=$pass fail=$fail"
cleanup_check() {
	[ "$had_coverage_out" -eq 0 ] && rm -f coverage.out
	diff -u "$SNAPSHOT" <(git status --porcelain) >"$TMP/status-diff.txt" 2>&1
}
if ! cleanup_check; then
	echo "FAIL: tree changed by the matrix run:"
	cat "$TMP/status-diff.txt"
	exit 1
fi
[ "$fail" -eq 0 ]
