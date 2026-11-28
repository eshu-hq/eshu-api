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

// loginStateGrantSQL loads every login-state grant migration: 004
// pins the serving grants, 005 adds the purge-only role. Tests scan
// all of them so a DELETE grant to a serving role in any file fails.
func loginStateGrantSQL(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, f := range []string{"004_login_state_least_privilege.sql", "005_login_state_retention_purge.sql"} {
		raw, err := os.ReadFile(filepath.Join("migrations", f))
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		out = append(out, string(raw))
	}
	return out
}

func TestLoginStateGrants_ServingRolesNeverDelete(t *testing.T) {
	t.Parallel()
	for _, got := range loginStateGrantSQL(t) {
		for _, line := range strings.Split(strings.ToUpper(got), "\n") {
			grant := strings.TrimSpace(line)
			if !strings.HasPrefix(grant, "GRANT ") {
				continue
			}
			if strings.Contains(grant, "DELETE") && (strings.Contains(grant, "ESHU_API_SESSION_RW") || strings.Contains(grant, "ESHU_API_SESSION_RO")) {
				t.Fatalf("serving roles must hold no DELETE grant, found %q", line)
			}
		}
	}
}

func TestMigration005_PurgeRoleScopedToLoginStates(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("migrations", "005_login_state_retention_purge.sql"))
	if err != nil {
		t.Fatalf("read 005: %v", err)
	}
	got := string(raw)
	upper := strings.ToUpper(got)
	if !strings.Contains(got, "eshu_api_session_purge") {
		t.Fatal("005 must declare the eshu_api_session_purge role")
	}
	if !strings.Contains(got, "WITH LOGIN") {
		t.Fatal("005 must grant LOGIN so the purge job can authenticate as its role")
	}
	found := false
	for _, line := range strings.Split(upper, "\n") {
		grant := strings.TrimSpace(line)
		if !strings.HasPrefix(grant, "GRANT ") || !strings.Contains(grant, "DELETE") {
			continue
		}
		found = true
		if !strings.Contains(grant, "LOGIN_STATES") || !strings.Contains(grant, "ESHU_API_SESSION_PURGE") {
			t.Fatalf("every DELETE grant must target login_states for the purge role only, found %q", line)
		}
		for _, table := range []string{"BROWSER_SESSIONS", "IDENTITY_API_TOKENS", "IDENTITY_SUBJECTS"} {
			if strings.Contains(grant, table) {
				t.Fatalf("purge role must hold no grant on %s, found %q", table, line)
			}
		}
		for _, forbidden := range []string{"TRUNCATE", "DROP", "ALTER", "CREATE"} {
			if strings.Contains(grant, forbidden) {
				t.Fatalf("purge role must hold no DDL grant, found %q in %q", forbidden, line)
			}
		}
	}
	if !found {
		t.Fatal("005 must grant DELETE on login_states to the purge role")
	}
	if !strings.Contains(upper, "STATEMENT_TIMEOUT") {
		t.Fatal("005 must pin a server-side statement timeout for the purge role")
	}
}

func TestMigration005_ExpiresAtIndex(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("migrations", "005_login_state_retention_purge.sql"))
	if err != nil {
		t.Fatalf("read 005: %v", err)
	}
	upper := strings.ToUpper(string(raw))
	if !strings.Contains(upper, "CREATE INDEX IF NOT EXISTS LOGIN_STATES_EXPIRES_AT_IDX") {
		t.Fatal("005 must create the login_states_expires_at_idx idempotently")
	}
	if !strings.Contains(upper, "ON LOGIN_STATES (EXPIRES_AT)") {
		t.Fatal("005 index must cover expires_at, the purge predicate column")
	}
	if !strings.HasPrefix(strings.TrimSpace(upper), "--") || !strings.Contains(upper, "BEGIN;") || !strings.Contains(upper, "COMMIT;") {
		t.Fatal("005 must run in a BEGIN/COMMIT transaction like its predecessors")
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
