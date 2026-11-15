// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"os"
	"strings"
	"testing"
)

// TestW8bNarrowTerm_LaneFileHygiene is the no-core-import grep proof
// at the source level: the lane files carry no dest-side store, port,
// or table symbol (the pins' graph, content, and store readers plus
// the out-of-pin grant and summary legs all stay core-side), no
// query/form/path parser, no request-URI sighting, no body-key
// sighting, no logger/tracer reference, and no serialization
// primitive or lock. A store handle, parse, log line, or lock added to
// api/w8bnarrow_terminate.go or api/w8bnarrow_wave.go fails here
// until it is removed — the stores-stay-core and opaque-bytes gates
// are enforced by this test, not by review alone.
// scripts/check-parity.sh gates the same list.
func TestW8bNarrowTerm_LaneFileHygiene(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"w8bnarrow_terminate.go", "w8bnarrow_wave.go"} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		body := string(raw)
		for _, forbidden := range []string{
			"postgres",
			"KubernetesCorrelationReadAdmission",
			"ObservabilityCoverageCorrelationReadAdmission",
			"WorkItemEvidenceReadAdmission",
			"SemanticEvidenceReadAdmission",
			"DependencyReadAdmission",
			"ImageReadAdmission",
			"GraphEntityInventoryReadAdmission",
			"TagHistoryReadAdmission",
			"RuntimeDeclarationReadAdmission",
			"CloudInventoryReadAdmission",
			"IncidentContextReadAdmission",
			"SSMHostInventoryReadAdmission",
			"ServiceCatalogCorrelationReadAdmission",
			"CodeownersOwnershipReadAdmission",
			"GraphQuery",
			"ContentStore",
			"ContentReader",
			"ReadStore",
			"database/sql",
			"sql.DB",
			"sql.Open",
			".Query(",
			"ParseForm(",
			"FormValue(",
			"PathValue(",
			".RequestURI",
			`"repo_id"`,
			`"limit"`,
			`"offset"`,
			`"incident_id"`,
			`"image"`,
			`"kind"`,
			"slog.",
			"tracing.",
			"span.",
			"json.Marshal",
			"sync.",
			"Mutex",
		} {
			if strings.Contains(body, forbidden) {
				t.Errorf("%s contains %q: lane must stay store-free, parse-free, log-free, and lock-free", file, forbidden)
			}
		}
	}
}
