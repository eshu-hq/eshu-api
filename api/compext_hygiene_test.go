// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"os"
	"strings"
	"testing"
)

// TestCompextTerm_LaneFileHygiene is the no-core-import grep proof
// at the source level: the lane files carry no dest-side store, port,
// or table symbol (the pin's registry reader stays core-side, and
// the out-of-pin mutation legs stay unpinned), no query/form/path
// parser, no request-URI sighting, no body-key sighting, no
// logger/tracer reference, and no serialization primitive or lock.
// It also fails the lane on any dest-local ESHU_COMPONENT_HOME
// sighting: dest never reads a dest-local component home — core
// reads its own registry on the forwarded call (Wave-1b FS
// condition). A store handle, parse, log line, lock, or component
// home added to api/compext_terminate.go or api/compext_wave.go
// fails here until it is removed — the stores-stay-core and
// no-dest-home gates are enforced by this test, not by review
// alone. scripts/check-parity.sh gates the same list.
func TestCompextTerm_LaneFileHygiene(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"compext_terminate.go", "compext_wave.go"} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		body := string(raw)
		for _, forbidden := range []string{
			"ESHU_COMPONENT_HOME",
			"ComponentExtensionReadAdmission",
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
			`"component_id"`,
			`"limit"`,
			`"offset"`,
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
