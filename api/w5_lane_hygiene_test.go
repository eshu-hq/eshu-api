// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"os"
	"strings"
	"testing"
)

// W5 lane-file hygiene (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#97). The no-core-import grep proof at the source
// level: the 8 W5 lane files carry no dest-side store, port, or table
// symbol (the pins' graph and content readers, the code-topic
// investigator, the live/ECS stores, and the explanation/readiness
// stores all stay core-side), no query/form/path parser, no
// request-URI sighting, no body-key sighting, no logger/tracer
// reference, and no serialization primitive or lock. A store handle,
// parse, log line, or lock added to any W5 lane file fails here until
// it is removed — the stores-stay-core and opaque-bytes gates are
// enforced by this test, not by review alone.
// scripts/check-parity.sh gates the same list.
func TestW5LaneFileHygiene(t *testing.T) {
	t.Parallel()

	files := []string{
		"entity_terminate.go",
		"entity_wave.go",
		"impact_terminate.go",
		"impact_wave.go",
		"story_terminate.go",
		"story_wave.go",
		"packet_terminate.go",
		"packet_wave.go",
	}
	forbidden := []string{
		"postgres",
		"EntityReadAdmission",
		"ImpactReadAdmission",
		"RepositoryStoryReadAdmission",
		"SupplyChainImpactPacketReadAdmission",
		"GraphQuery",
		"ContentStore",
		"ContentReader",
		"ReadStore",
		"GetEntityContent",
		"SearchEntitiesByName",
		"ListRepoFiles",
		"MatchRepositories",
		"RunSingle",
		"InvestigateCodeTopic",
		"ListRunningTasks",
		"ListLiveIdentityMatches",
		"HasLiveIdentityMatch",
		"ExplainSupplyChainImpact",
		"ReadSupplyChainImpactReadiness",
		"database/sql",
		"sql.DB",
		"sql.Open",
		".Query(",
		"ParseForm(",
		"FormValue(",
		"PathValue(",
		".RequestURI",
		`"Location"`,
		`"Set-Cookie"`,
		`"entity_id"`,
		`"service_name"`,
		`"repo_id"`,
		`"scope_id"`,
		`"topic"`,
		`"max_depth"`,
		`"limit"`,
		`"offset"`,
		"slog.",
		"tracing.",
		"span.",
		"json.Marshal",
		"sync.",
		"Mutex",
	}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		body := string(raw)
		for _, banned := range forbidden {
			if strings.Contains(body, banned) {
				t.Errorf("%s contains %q: lane must stay store-free, parse-free, log-free, and lock-free", file, banned)
			}
		}
	}
}
