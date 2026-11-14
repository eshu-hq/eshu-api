// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"encoding/json"
	"os"
	"sort"
	"testing"
)

// Path B served-route drift gate (eshu-hq/eshu-api#69, part of
// eshu-hq/eshu#9669). The openapi-level gate pins the assembled
// document to the committed snapshot; this gate pins the document as
// served through the real HTTP route (both core-origin postures) to
// the same snapshot file. Both gates consume the committed file plus
// in-process serving only — never a live fetch, which would measure a
// deployment rather than the committed tree.

// routeSnapshot carries the snapshot fields this gate checks. The file
// is owned by the openapi package (openapi/spec_snapshot.json); the
// relative path holds because Go runs package tests with the package
// directory as the working directory.
type routeSnapshot struct {
	SpecVersion      string   `json:"spec_version"`
	Served           int      `json:"served"`
	ServedOperations []string `json:"served_operations"`
}

// TestSpecSnapshot_ServedMatchesSnapshot pins that the document served
// through GET /api/v0/openapi.json equals the committed snapshot
// operation-for-operation, for both a configured and an unconfigured
// core origin, and that the served count reconciles with the scope
// proof (mounted minus disclosed absences).
func TestSpecSnapshot_ServedMatchesSnapshot(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../openapi/spec_snapshot.json")
	if err != nil {
		t.Fatalf("read committed spec snapshot: %v", err)
	}
	var snap routeSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatalf("committed spec snapshot is not valid JSON: %v", err)
	}

	want := append([]string(nil), snap.ServedOperations...)
	sort.Strings(want)
	if len(want) != snap.Served {
		t.Fatalf("snapshot lists %d operations, want served %d", len(want), snap.Served)
	}
	if reconciled := len(specScopeMounted()) - len(specScopeAbsent); reconciled != snap.Served {
		t.Fatalf("snapshot served = %d, want mounted-minus-absent %d", snap.Served, reconciled)
	}

	for _, origin := range []string{"http://core.invalid", ""} {
		served := specScopeServed(t, origin)
		if len(served) != snap.Served {
			t.Errorf("origin %q: served operations = %d, want snapshot %d", origin, len(served), snap.Served)
		}
		for _, key := range want {
			if !served[key] {
				t.Errorf("origin %q: snapshot operation %s is not served through the route", origin, key)
			}
		}
		for key := range served {
			found := sort.SearchStrings(want, key) < len(want) && want[sort.SearchStrings(want, key)] == key
			if !found {
				t.Errorf("origin %q: served operation %s is missing from the snapshot: re-pin with the owning lane", origin, key)
			}
		}
	}
}
