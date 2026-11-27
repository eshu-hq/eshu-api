// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package sessionstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// migrationSQL loads the least-privilege role migration shipped with
// this package. The test runs from the package directory, so the
// relative path holds both locally and in CI.
func migrationSQL(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("migrations", "002_session_least_privilege_roles.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	return string(raw)
}

func TestMigration_RolesCanLogIn(t *testing.T) {
	// Refs eshu-hq/eshu-api#447 review: the pool authenticates AS
	// these roles, so the migration must grant LOGIN (passwords are
	// set out-of-band via the secret store, never in this file).
	// NOLOGIN roles would refuse password authentication.
	sql := migrationSQL(t)
	if strings.Contains(sql, "WITH NOLOGIN") {
		t.Fatal("session roles must carry LOGIN so the pool can authenticate as them")
	}
	if !strings.Contains(sql, "WITH LOGIN") {
		t.Fatal("migration must grant LOGIN to both session roles")
	}
}

func TestMigration_DeclaresBothLeastPrivilegeRoles(t *testing.T) {
	t.Parallel()
	got := migrationSQL(t)
	for _, want := range []string{"eshu_api_session_rw", "eshu_api_session_ro"} {
		if !strings.Contains(got, want) {
			t.Fatalf("migration must declare role %q", want)
		}
	}
}

func TestMigration_RWRoleCannotDeleteOrDDL(t *testing.T) {
	t.Parallel()
	got := strings.ToUpper(migrationSQL(t))
	// Explanatory comments name the forbidden operations, so only
	// actual GRANT lines are checked: no DELETE/TRUNCATE/DDL grant
	// may appear. (ALTER ROLE ... SET statement_timeout is a
	// guardrail, not schema DDL.)
	for _, line := range strings.Split(got, "\n") {
		grant := strings.TrimSpace(strings.ToUpper(line))
		if !strings.HasPrefix(grant, "GRANT ") {
			continue
		}
		for _, forbidden := range []string{"DELETE", "TRUNCATE", "DROP", "ALTER", "CREATE"} {
			if strings.Contains(grant, forbidden) {
				t.Fatalf("runtime roles must hold no DELETE/DDL grants, found %q in %q", forbidden, line)
			}
		}
	}
}

func TestMigration_RORoleIsSelectOnly(t *testing.T) {
	t.Parallel()
	got := migrationSQL(t)
	upper := strings.ToUpper(got)
	ro := upper[strings.Index(upper, "ESHU_API_SESSION_RO"):]
	for _, forbidden := range []string{"INSERT", "UPDATE", "DELETE"} {
		if strings.Contains(ro, "GRANT "+forbidden) {
			t.Fatalf("read-only role must not be granted %s", forbidden)
		}
	}
	if !strings.Contains(ro, "GRANT SELECT") {
		t.Fatal("read-only role must hold SELECT on the profile/list surface")
	}
}

func TestMigration_RevokesPublicFirst(t *testing.T) {
	t.Parallel()
	got := strings.ToUpper(migrationSQL(t))
	if !strings.Contains(got, "REVOKE ALL") {
		t.Fatal("migration must revoke public/default grants before granting least privilege")
	}
}

func TestMigration_SetsStatementTimeout(t *testing.T) {
	t.Parallel()
	got := strings.ToUpper(migrationSQL(t))
	if !strings.Contains(got, "STATEMENT_TIMEOUT") {
		t.Fatal("migration must pin a server-side statement timeout for both roles")
	}
}

func TestMigration_PinsPoolCeilingsAsComment(t *testing.T) {
	t.Parallel()
	got := migrationSQL(t)
	if !strings.Contains(got, "MaxReplicas") {
		t.Fatal("migration must record the global pool-arithmetic formula so operators size max_connections from it")
	}
}

func TestMigration_ClosesFutureTableGrants(t *testing.T) {
	t.Parallel()
	got := strings.ToUpper(migrationSQL(t))
	if !strings.Contains(got, "ALTER DEFAULT PRIVILEGES") {
		t.Fatal("migration must revoke default privileges so future tables grant nothing to the session roles")
	}
}

func TestTables_CreateSessionIdentityTokenSurfaces(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("migrations", "001_session_tables.sql"))
	if err != nil {
		t.Fatalf("read tables migration: %v", err)
	}
	got := strings.ToUpper(string(raw))
	for _, table := range []string{"BROWSER_SESSIONS", "IDENTITY_API_TOKENS", "IDENTITY_SUBJECTS"} {
		if !strings.Contains(got, "CREATE TABLE IF NOT EXISTS "+table) {
			t.Fatalf("001 must create %s idempotently before 002 grants on it", table)
		}
	}
}
