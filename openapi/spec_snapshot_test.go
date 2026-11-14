// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

// Path B scoped-inventory drift gate (eshu-hq/eshu-api#69, part of
// eshu-hq/eshu#9669). The satellite serves its own mounted scope only,
// so the served document can never bijecct the full core surface (Path
// A). Instead the served scope is pinned in the committed snapshot file
// spec_snapshot.json and this gate fails closed on any drift: a lane
// that ports, renames, or removes a served operation updates the
// snapshot with its lane. The gate consumes the committed file plus the
// in-process Spec() assembly only — never a live fetch, which would
// measure a deployment rather than the committed tree.

// snapshotMethods is the served-operation method set, matching the
// api-side scope proof (path-item metadata never counts).
var snapshotMethods = map[string]bool{
	"get": true, "post": true, "put": true, "patch": true,
	"delete": true, "head": true, "options": true,
}

// specSnapshot is the committed served-scope pin. The operations list
// is the sorted Method+Path set the document served when the snapshot
// was taken; specSHA256 pins the full document bytes so content drift
// (summaries, schemas, markers) fails here too, not just scope drift.
type specSnapshot struct {
	SnapshotVersion  int      `json:"snapshot_version"`
	SpecVersion      string   `json:"spec_version"`
	Source           string   `json:"source"`
	Mounted          int      `json:"mounted"`
	DisclosedAbsent  int      `json:"disclosed_absent"`
	Served           int      `json:"served"`
	SpecSHA256       string   `json:"spec_sha256"`
	ServedOperations []string `json:"served_operations"`
}

// loadSpecSnapshot reads the committed pin and fails closed on any
// read or shape defect: a missing or corrupt snapshot is drift, not a
// skip.
func loadSpecSnapshot(t *testing.T) specSnapshot {
	t.Helper()
	raw, err := os.ReadFile("spec_snapshot.json")
	if err != nil {
		t.Fatalf("read committed spec snapshot: %v (regenerate it from Spec(), never from a live fetch)", err)
	}
	var snap specSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatalf("committed spec snapshot is not valid JSON: %v", err)
	}
	if snap.SnapshotVersion != 1 {
		t.Fatalf("snapshot_version = %d, want 1", snap.SnapshotVersion)
	}
	if snap.SpecVersion != SpecVersion {
		t.Fatalf("snapshot spec_version = %q, want SpecVersion %q (bump the snapshot with the serving lane)", snap.SpecVersion, SpecVersion)
	}
	return snap
}

// specServedOps derives the served Method+Path set from the in-process
// Spec() assembly, sorted for a stable comparison.
func specServedOps(t *testing.T) []string {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(Spec()), &doc); err != nil {
		t.Fatalf("Spec() is not valid JSON: %v", err)
	}
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("Spec() paths = %T, want object", doc["paths"])
	}
	var ops []string
	for path, raw := range paths {
		item, _ := raw.(map[string]any)
		for method := range item {
			if snapshotMethods[strings.ToLower(method)] {
				ops = append(ops, strings.ToUpper(method)+" "+path)
			}
		}
	}
	sort.Strings(ops)
	return ops
}

// TestSpecSnapshot_NoDrift pins the served scope and document bytes to
// the committed snapshot: any served-operation or content change fails
// here until the owning lane re-pins the snapshot file.
func TestSpecSnapshot_NoDrift(t *testing.T) {
	t.Parallel()

	snap := loadSpecSnapshot(t)
	served := specServedOps(t)

	if got := len(served); got != snap.Served {
		t.Errorf("served operations = %d, want snapshot %d: re-pin spec_snapshot.json with the owning lane", got, snap.Served)
	}
	if len(snap.ServedOperations) != snap.Served {
		t.Errorf("snapshot lists %d operations, want served %d: the snapshot file itself drifted", len(snap.ServedOperations), snap.Served)
	}
	want := append([]string(nil), snap.ServedOperations...)
	sort.Strings(want)
	for i := range served {
		if i >= len(want) {
			break
		}
		if served[i] != want[i] {
			t.Errorf("served operation mismatch at index %d: got %q, want snapshot %q", i, served[i], want[i])
		}
	}
	if len(served) != len(want) {
		t.Errorf("served operation count = %d, snapshot count = %d", len(served), len(want))
	}

	sum := sha256.Sum256([]byte(Spec()))
	if got := hex.EncodeToString(sum[:]); got != snap.SpecSHA256 {
		t.Errorf("Spec() sha256 = %s, want snapshot %s: document content drifted, re-pin with the owning lane", got, snap.SpecSHA256)
	}
}

// TestSpecSnapshot_Sorted pins the snapshot file canonical form: the
// committed list stays sorted so diffs review operation-by-operation.
func TestSpecSnapshot_Sorted(t *testing.T) {
	t.Parallel()

	snap := loadSpecSnapshot(t)
	if !sort.StringsAreSorted(snap.ServedOperations) {
		t.Errorf("committed snapshot operations are not sorted: keep the file canonical")
	}
	seen := map[string]bool{}
	for _, op := range snap.ServedOperations {
		if seen[op] {
			t.Errorf("committed snapshot lists %q twice", op)
		}
		seen[op] = true
	}
}
