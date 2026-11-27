// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package sessionstore

import (
	"context"
	"time"
)

// AuthContext is the eshu-api-local authentication verdict a
// resolved browser session yields. It mirrors the fail-closed shape
// of the v0 SessionResolverAdmission pin: the middleware denies with
// 401 whenever ok is false or err is non-nil, and never invents a
// context. Only hashes and scope flags cross; no secret material.
type AuthContext struct {
	// SubjectIDHash identifies the subject without exposing it.
	SubjectIDHash string
	// TenantID and WorkspaceID scope the session.
	TenantID    string
	WorkspaceID string
	// AllScopes reports an all-scopes session; only those may
	// switch workspace.
	AllScopes bool
	// ExpiresAt bounds the verdict in time.
	ExpiresAt time.Time
}

// BrowserSessionCreate carries the hash-only record a new browser
// session persists. The plaintext session and CSRF tokens stay with
// the cookie layer; only their hashes reach the store.
type BrowserSessionCreate struct {
	SessionHash       string
	CSRFHash          string
	SubjectIDHash     string
	TenantID          string
	WorkspaceID       string
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
}

// BrowserSessionListItem is one metadata-only session row. It
// deliberately carries no session, CSRF, or secret hash: the store
// computes Current by hash comparison server-side.
type BrowserSessionListItem struct {
	TenantID          string
	WorkspaceID       string
	IssuedAt          time.Time
	LastSeenAt        time.Time
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
	Current           bool
}

// APITokenCreate carries the hash-only record a generated token
// persists. The bearer plaintext is returned to the caller once
// only; the store keeps the hash.
type APITokenCreate struct {
	TokenID       string
	TokenHash     string
	SubjectIDHash string
	Name          string
	ExpiresAt     time.Time
}

// APITokenRevoke scopes a revocation by token ID with atomic
// ownership: the subject hash must match the row owner.
type APITokenRevoke struct {
	TokenID       string
	SubjectIDHash string
}

// APITokenRotate atomically swaps a token hash under the same
// ownership predicate as revocation.
type APITokenRotate struct {
	TokenID       string
	SubjectIDHash string
	NewTokenHash  string
}

// APITokenListItem is one metadata-only token row: no token_hash.
type APITokenListItem struct {
	TokenID   string
	Name      string
	CreatedAt time.Time
	ExpiresAt time.Time
	Revoked   bool
}

// MFAStatus is the caller MFA-status read behind the profile route:
// enrollment facts only, never credential material.
type MFAStatus struct {
	Enrolled   bool
	VerifiedAt time.Time
}

// Resolver is the eshu-api-local port for the v0
// SessionResolverAdmission pin: ResolveBrowserSession only.
type Resolver interface {
	ResolveBrowserSession(ctx context.Context, sessionHash, csrfTokenHash string, requireCSRF bool, now time.Time) (AuthContext, bool, error)
}

// BrowserSessions is the eshu-api-local port for the v0
// BrowserSessionAdmission pin: lifecycle plus metadata-only list.
type BrowserSessions interface {
	CreateBrowserSession(ctx context.Context, record BrowserSessionCreate) error
	RevokeBrowserSession(ctx context.Context, sessionHash string, now time.Time) error
	SwitchBrowserSessionWorkspace(ctx context.Context, sessionHash, tenantID, workspaceID string, now time.Time) (AuthContext, bool, error)
	ListSessionsBySubject(ctx context.Context, subjectIDHash string, asOf time.Time, sessionHash string, limit, offset int) ([]BrowserSessionListItem, error)
}

// IdentityProfiles is the eshu-api-local port for the read-only
// subset of the v0 LocalIdentityAdmission pin this issue wires: the
// profile MFA-status read and the subject-ID resolve. Password/MFA
// verification writes stay unwired until P5/P6.
type IdentityProfiles interface {
	GetLocalIdentityMFAStatus(ctx context.Context, subjectIDHash string, now time.Time) (MFAStatus, error)
	ResolveLocalIdentityUserID(ctx context.Context, subjectIDHash string) (string, bool, error)
}

// APITokens is the eshu-api-local port for the v0 APITokenAdmission
// pin: create, metadata-only list, revoke, rotate.
type APITokens interface {
	CreateLocalIdentityAPIToken(ctx context.Context, record APITokenCreate) error
	ListAPITokensBySubject(ctx context.Context, subjectIDHash string, asOf time.Time, limit, offset int) ([]APITokenListItem, error)
	RevokeLocalIdentityAPIToken(ctx context.Context, revocation APITokenRevoke) error
	RotateLocalIdentityAPIToken(ctx context.Context, rotation APITokenRotate) error
}
