// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"os"
	"strings"
	"testing"
)

// pinnedVersion reads a version pin from a vendored contract document.
// It finds the declaration line for symbol (of the form SYMBOL = "vX")
// and returns the quoted value. It fails the test when the file or the
// line is missing, so a deleted or relocated pin cannot pass silently.
func pinnedVersion(t *testing.T, docPath, symbol string) string {
	t.Helper()

	raw, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read vendored pin %s: %v", docPath, err)
	}
	for line := range strings.Lines(string(raw)) {
		idx := strings.Index(line, symbol+" = \"")
		if idx < 0 {
			continue
		}
		rest := line[idx+len(symbol+" = \""):]
		end := strings.Index(rest, "\"")
		if end < 0 {
			t.Fatalf("pin line for %s in %s has no closing quote: %q", symbol, docPath, line)
		}
		return rest[:end]
	}
	t.Fatalf("no %s pin line in %s", symbol, docPath)
	return ""
}

// tablePinnedVersion reads a version pin from a Markdown table row of
// the form | `SYMBOL` | `vX` | (docs/provenance.md). It fails the test
// when the row is missing, so a dropped pin row cannot pass silently.
func tablePinnedVersion(t *testing.T, docPath, symbol string) string {
	t.Helper()

	raw, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read vendored pin %s: %v", docPath, err)
	}
	for line := range strings.Lines(string(raw)) {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		if !strings.Contains(line, "`"+symbol+"`") {
			continue
		}
		rest := line[strings.Index(line, "`"+symbol+"`")+len(symbol)+2:]
		start := strings.Index(rest, "`")
		if start < 0 {
			continue
		}
		rest = rest[start+1:]
		end := strings.Index(rest, "`")
		if end < 0 {
			t.Fatalf("pin row for %s in %s has no closing backtick: %q", symbol, docPath, line)
		}
		return rest[:end]
	}
	t.Fatalf("no %s pin row in %s", symbol, docPath)
	return ""
}

// TestContractVersionsArePinned locks the skeleton to the contract
// versions vendored under contracts/. Each assertion reads the pin from
// the document and compares it to the package constant, so the test
// fails in both drift directions: a bumped document with a stale
// constant, and a bumped constant with a stale document. Consuming a
// newer core contract requires recording the new pin in
// docs/provenance.md first and bumping the constant in the same diff.
func TestContractVersionsArePinned(t *testing.T) {
	t.Parallel()

	if got := pinnedVersion(t, "../contracts/core-read-contracts.md", "CoreReadContractsVersion"); got != CoreReadContractsVersion {
		t.Errorf("contracts/core-read-contracts.md pins %q, constant is %q: move them in the same diff", got, CoreReadContractsVersion)
	}
	if got := pinnedVersion(t, "../contracts/suppression-admission.md", "SuppressionAdmissionContractVersion"); got != SuppressionAdmissionContractVersion {
		t.Errorf("contracts/suppression-admission.md pins %q, constant is %q: move them in the same diff", got, SuppressionAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "CoreReadContractsVersion"); got != CoreReadContractsVersion {
		t.Errorf("docs/provenance.md pins core %q, constant is %q: record the new pin before consuming it", got, CoreReadContractsVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "SuppressionAdmissionContractVersion"); got != SuppressionAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins admission %q, constant is %q: record the new pin before consuming it", got, SuppressionAdmissionContractVersion)
	}
}
