# Local parity for the CI lanes lint/test/race. CI adds parity, shell,
# govulncheck, gosec, and actionlint; see .github/workflows/ci.yml.
# Needs: go (version in go.mod), golangci-lint v2.12.2
# (same binary Eshu uses), git (for the 500-line cap file list).
#
# Skeleton state: go.mod has landed, so every Go target below runs for
# real. The contracts/upstream/ tree never reaches them: its nested go.mod
# marker keeps the module toolchain from loading parity evidence as build
# inputs.

# Measured baseline from the first Go PR (HTTP skeleton, review round 3):
# 88.7% of statements. The floor may only rise toward the 90% finish line.
COVERAGE_FLOOR ?= 88.7

.PHONY: ci parity lint test race

ci: parity lint test race

parity:
	bash scripts/check-parity.sh

lint:
	@if [ -f go.mod ]; then \
	  go vet ./...; \
	  golangci-lint run ./...; \
	  golangci-lint fmt --diff ./...; \
	  go mod tidy -diff; \
	  over=0; for f in $$(git ls-files '*.go'); do n=$$(wc -l < "$$f"); if [ "$$n" -gt 500 ]; then echo "$$f: $$n lines"; over=1; fi; done; test "$$over" -eq 0; \
	else echo "lint: no go.mod yet, dormant (Refs eshu-hq/eshu-api#1)"; fi

test:
	@if [ -f go.mod ]; then \
	  go build ./...; \
	  go test ./... -count=1; \
	else echo "test: no go.mod yet, dormant (Refs eshu-hq/eshu-api#1)"; fi

race:
	@if [ -f go.mod ]; then \
	  go test ./... -race -count=1 -covermode=atomic -coverprofile=coverage.out; \
	  go tool cover -func=coverage.out; \
	  total=$$(go tool cover -func=coverage.out | awk '/^total:/ {gsub(/%/, "", $$3); print $$3}'); echo "statement coverage total=$${total}% floor=$(COVERAGE_FLOOR)%"; awk -v t="$$total" -v f="$(COVERAGE_FLOOR)" 'BEGIN { if (t + 0 < f + 0) { print "FAIL: coverage below floor"; exit 1 } }'; \
	else echo "race: no go.mod yet, dormant (Refs eshu-hq/eshu-api#1)"; fi
