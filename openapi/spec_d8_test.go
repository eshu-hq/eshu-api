// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"testing"
)

// D8 session-resolver spec proof (Refs eshu-hq/eshu#9669, part of
// #6707). The D8 lane ports five line-verbatim subsets (BrowserSessions,
// SessionList, AuthProfile, LocalIdentity, plus the invitations POST
// merged into AdminInvitations) and two whole-file byte-identical ports
// (Tokens, TOTP) into the served document, plus the 11 schemas those
// entries reference. Byte identity of every ported line rides
// scripts/check-parity.sh; these tests pin the served assembly around
// those bytes.
//
// New $refs the D8 lane introduces beyond the existing components
// block: the 11 D8 schemas (ported alongside the fragments, so the
// generic TestSpec_LocalRefsResolve covers them without edits). The
// entries name no other new component.

// TestSpec_ServesD8Families pins the native-spec contract for the D8
// session-resolver families: the assembled document is valid JSON and
// carries every lane path key with its pinned summary, and exactly the
// methods each fragment entry documents (the invitations key carries
// the D3 GET plus the D8 POST).
func TestSpec_ServesD8Families(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	for path, wantOps := range map[string]map[string]string{
		"/api/v0/auth/browser-session": {
			"post":   "Create a dashboard browser session",
			"get":    "Read the current dashboard browser session",
			"delete": "Revoke the current dashboard browser session",
		},
		"/api/v0/auth/browser-session/context": {
			"patch": "Switch the current dashboard session tenant/workspace",
		},
		"/api/v0/auth/sessions": {
			"get": "List the caller's own browser sessions",
		},
		"/api/v0/auth/profile": {
			"get": "Read the caller's identity profile",
		},
		"/api/v0/auth/local/bootstrap": {
			"post": "Bootstrap the first local identity owner",
		},
		"/api/v0/auth/local/login": {
			"post": "Authenticate a local identity",
		},
		"/api/v0/auth/local/invitations": {
			"post": "Create a local identity invitation",
			"get":  "List the tenant's invitations",
		},
		"/api/v0/auth/local/invitations/accept": {
			"post": "Accept a local identity invitation",
		},
		"/api/v0/auth/local/users/{user_id}/password": {
			"post": "Reset a local identity password",
		},
		"/api/v0/auth/local/password/rotate": {
			"post": "Self-service local identity password rotation",
		},
		"/api/v0/auth/local/users/{user_id}/mfa-reset": {
			"post": "Reset local identity MFA recovery material",
		},
		"/api/v0/auth/local/users/{user_id}/disable": {
			"post": "Disable a local identity",
		},
		"/api/v0/auth/local/break-glass": {
			"post": "Enable time-boxed local identity break-glass recovery",
		},
		"/api/v0/auth/local/break-glass/session": {
			"post": "Create a browser session from break-glass recovery",
		},
		"/api/v0/auth/local/api-tokens": {
			"get":  "List the caller's generated API tokens",
			"post": "Create a generated API token",
		},
		"/api/v0/auth/local/api-tokens/{token_id}/revoke": {
			"post": "Revoke a generated API token",
		},
		"/api/v0/auth/local/api-tokens/{token_id}/rotate": {
			"post": "Rotate a generated API token",
		},
		"/api/v0/auth/local/mfa/totp/begin": {
			"post": "Begin authenticator-app (TOTP) MFA enrollment",
		},
		"/api/v0/auth/local/mfa/totp/confirm": {
			"post": "Confirm authenticator-app (TOTP) MFA enrollment",
		},
	} {
		item, ok := served[path].(map[string]any)
		if !ok {
			t.Errorf("paths lacks %s: %v", path, keys(served))
			continue
		}
		if len(item) != len(wantOps) {
			t.Errorf("%s serves %d methods, want %d (%v)", path, len(item), len(wantOps), keys(item))
			continue
		}
		for method, wantSummary := range wantOps {
			op, _ := item[method].(map[string]any)
			if got := op["summary"]; got != wantSummary {
				t.Errorf("%s %s summary = %v, want %q", path, method, got, wantSummary)
			}
		}
	}
}

// TestSpec_D8FamilySectionMatchesFragment pins lossless assembly for
// the D8 keys: the served path objects equal the ported fragments'
// parsed path objects, so the only reviewed delta to the pinned core
// fragments is the fragment files themselves (gated line-verbatim by
// scripts/check-parity.sh).
func TestSpec_D8FamilySectionMatchesFragment(t *testing.T) {
	t.Parallel()

	served := decodePaths(t, Spec())
	fragment := decodePaths(t, `{"paths": {`+familyPaths()+`  }}`)

	for _, path := range []string{
		"/api/v0/auth/browser-session",
		"/api/v0/auth/browser-session/context",
		"/api/v0/auth/sessions",
		"/api/v0/auth/profile",
		"/api/v0/auth/local/bootstrap",
		"/api/v0/auth/local/login",
		"/api/v0/auth/local/invitations",
		"/api/v0/auth/local/invitations/accept",
		"/api/v0/auth/local/users/{user_id}/password",
		"/api/v0/auth/local/password/rotate",
		"/api/v0/auth/local/users/{user_id}/mfa-reset",
		"/api/v0/auth/local/users/{user_id}/disable",
		"/api/v0/auth/local/break-glass",
		"/api/v0/auth/local/break-glass/session",
		"/api/v0/auth/local/api-tokens",
		"/api/v0/auth/local/api-tokens/{token_id}/revoke",
		"/api/v0/auth/local/api-tokens/{token_id}/rotate",
		"/api/v0/auth/local/mfa/totp/begin",
		"/api/v0/auth/local/mfa/totp/confirm",
	} {
		want, ok := fragment[path]
		if !ok {
			t.Fatalf("fragment lacks %s", path)
		}
		got, ok := served[path]
		if !ok {
			t.Fatalf("served spec lacks %s", path)
		}
		if !jsonEqual(got, want) {
			t.Errorf("served %s differs from the ported fragment", path)
		}
	}
}
