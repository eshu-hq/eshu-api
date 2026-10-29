// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import "testing"

// TestContractVersionsArePinned locks the skeleton to the contract
// versions vendored under contracts/. Consuming a newer core contract
// requires recording the new pin in docs/provenance.md first and bumping
// these constants in the same diff, so a silent drift can never ship.
func TestContractVersionsArePinned(t *testing.T) {
	t.Parallel()

	if CoreReadContractsVersion != "v0" {
		t.Errorf("CoreReadContractsVersion = %q, want %q: bump it deliberately with the port change", CoreReadContractsVersion, "v0")
	}
	if SuppressionAdmissionContractVersion != "v0" {
		t.Errorf("SuppressionAdmissionContractVersion = %q, want %q: bump it deliberately with the port change", SuppressionAdmissionContractVersion, "v0")
	}
}
