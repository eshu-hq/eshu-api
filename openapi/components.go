// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

// components is the minimal shared block for the dest-served spec:
// exactly the schemas and responses the landed family fragments
// reference, nothing more. The block lives in components.json,
// embedded verbatim so the served bytes never depend on Go file
// layout (the 500-line file cap counts only *.go). Entry bytes are
// ported from the pinned core
// commits 428790ab3ac0bff2eb28e50a26b3746a9b7dc8c5 (sign-in-policy lane)
// and ebaf4dfa0de672851bbec7a427e28fdb41344380 (retirement,
// terminal-failure, and admin-identity lanes),
// 99381691b1ac49254e813e5a013cfb654d432491 (provider-config lane),
// and 7c60583996d9a3ccc46288f40a38f614a4f28ea8 (recovery lane).
// The terminal-failure lane adds no entries: its fragment references
// only ErrorResponse plus the five responses above (gated in
// check-parity.sh):
//
//   - AdminSignInPolicy, AdminSignInPolicyUpdateRequest: schemas from
//     go/internal/query/openapi/components_sign_in_policy.go.
//   - AdminAuditCountList: schema from
//     go/internal/query/openapi/components_local_identity.go (admin-identity
//     reads lane: the audit-summary fragment response counts by it).
//   - AdminProviderConfigWriteResult: schema from
//     go/internal/query/openapi/components_provider_configs.go.
//   - SkippedScopesReport: schema from
//     go/internal/query/openapi/components.go (recovery lane: the
//     refinalize and recover-generations entries reference it, and no
//     relay-only route does).
//   - DeltaActiveScopesReport, ReindexRequestsWrittenReport: schemas
//     from go/internal/query/openapi/components_recovery.go (same
//     lane, same two entries, no relay-only reference).
//   - ErrorResponse: schema from go/internal/query/openapi/components.go.
//   - BadRequest, NotFound, Forbidden, Conflict, InternalError,
//     ServiceUnavailable: responses from
//     go/internal/query/openapi/components_responses.go.
//   - RepoId: parameter from go/internal/query/openapi/components.go
//     (D4 Native-next wave: the served repository entries address
//     their routes by it).
//   - CollectorReadinessEnvelope, Repository, RepositoryRef,
//     CatalogWorkload, InvestigationEvidencePacket, Relationship:
//     schemas from go/internal/query/openapi/components.go (D4 wave:
//     referenced by the served CI/CD, repository, evidence, and
//     investigation entries).
//   - ActiveWorkSource: schema from
//     go/internal/query/openapi/components_active_work_source.go (D4
//     wave: referenced by the served evidence bundle entry).
//   - GatewayTimeout: response from
//     go/internal/query/openapi/components_responses.go (D4 wave:
//     answered by the served cloud, infra, repository,
//     package-registry, and evidence-route entries).
//   - CoordinatorSource, TerraformStateSource: schemas from
//     go/internal/query/openapi/components_active_work_source.go (D6
//     wave: referenced by the served status pipeline, ingesters, and
//     collectors entries).
//   - LocalIdentityBootstrapRequest, LocalIdentityLoginRequest,
//     LocalIdentityInvitationRequest,
//     LocalIdentityInvitationAcceptanceRequest,
//     LocalIdentityPasswordRotationRequest, LocalIdentityBreakGlassRequest,
//     LocalIdentityAPITokenCreateRequest, LocalIdentityAPITokenResponse,
//     LocalIdentitySessionResponse: schemas from
//     go/internal/query/openapi/components_local_identity.go (D8 wave:
//     referenced by the served local-identity entries).
//   - BrowserSessionAuth, BrowserSessionResponse: schemas from
//     go/internal/query/openapi/components_workload_session.go (D8 wave:
//     referenced by the served browser-session and local-identity
//     session entries).
//   - FileContent, EntityContent: schemas from
//     go/internal/query/openapi/components.go (Wave-2 Content wave:
//     referenced by the served file-read, file-lines, and entity-read
//     entries).
//
// A new family lane extends this block with the entries its fragment
// references; TestSpec_LocalRefsResolve fails closed on a dangling $ref.
// Every D4 entry is line-verbatim in its pinned source at commit
// 17feac86e0 (gated by check_d4_components_at in
// scripts/check-parity.sh). Every D6 entry is line-verbatim in its
// pinned source at commit 6b66e6b23efe226970b4571c955aed1dc42f987a
// (gated by check_d6_components_at in scripts/check-parity.sh).

import _ "embed"

//go:embed components.json
var components string
