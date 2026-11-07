// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package auth

// LocalIdentity documents the D8 local-identity lifecycle lane (Refs
// eshu-hq/eshu#9669): the bootstrap, login, invitations-accept,
// password, password-rotation, MFA-reset, disable, break-glass, and
// break-glass-session POSTs (tier-C dispatch clones, not termination).
// The invitations-create POST for the shared invitations key lives in
// the AdminInvitations fragment, which D8 extends. Each payload line
// below is verbatim from the pinned core file
// go/internal/query/openapi/paths/auth/routes.go
// at commit 91f2df28dbe9ec4e5fb8c5df59a76475a304d84 (sha256
// 709fe6b3e96967fa615bff59a8c02cb8c1c5fbc6a05744bf57835170a5ad633a,
// byte-identical there and at the D2 wave pin). Same subset treatment
// as the BrowserSessions fragment, gated by scripts/check-parity.sh.
const LocalIdentity = `
    "/api/v0/auth/local/bootstrap": {
      "post": {
        "tags": ["auth"],
        "summary": "Bootstrap the first local identity owner",
        "description": "Operator-controlled one-time setup for deployments that do not use an external IdP. This route is not public: it requires the shared operator bearer token, stores only hashed login/recovery identifiers plus a bcrypt password hash, creates the first owner role assignment, and requires MFA recovery-code material for the admin account.",
        "operationId": "bootstrapLocalIdentity",
        "requestBody": {
          "required": true,
          "content": {"application/json": {"schema": {"$ref": "#/components/schemas/LocalIdentityBootstrapRequest"}}}
        },
        "responses": {
          "201": {"description": "First owner/admin created."},
          "400": {"$ref": "#/components/responses/BadRequest"},
          "403": {"$ref": "#/components/responses/Forbidden"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/auth/local/login": {
      "post": {
        "tags": ["auth"],
        "summary": "Authenticate a local identity",
        "description": "Public local-login route. The handler hashes the submitted login id and optional recovery code before storage lookup, verifies the password against the stored bcrypt hash, enforces MFA for admin accounts, applies lockout state, and creates the same secure browser-session cookies used by the dashboard when authentication succeeds.",
        "operationId": "loginLocalIdentity",
        "requestBody": {
          "required": true,
          "content": {"application/json": {"schema": {"$ref": "#/components/schemas/LocalIdentityLoginRequest"}}}
        },
        "responses": {
          "200": {"description": "Authenticated and browser session created.", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/LocalIdentitySessionResponse"}}}},
          "202": {"description": "Password accepted but MFA proof is required.", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/LocalIdentitySessionResponse"}}}},
          "401": {"$ref": "#/components/responses/Unauthorized"},
          "403": {"$ref": "#/components/responses/Forbidden"},
          "423": {"description": "Local identity is temporarily locked."},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/auth/local/invitations/accept": {
      "post": {
        "tags": ["auth"],
        "summary": "Accept a local identity invitation",
        "description": "Public invitation acceptance route. A valid active invite code is required; the route stores only invite/login/recovery hashes plus a bcrypt password hash.",
        "operationId": "acceptLocalIdentityInvitation",
        "requestBody": {
          "required": true,
          "content": {"application/json": {"schema": {"$ref": "#/components/schemas/LocalIdentityInvitationAcceptanceRequest"}}}
        },
        "responses": {
          "201": {"description": "Invitation accepted and local identity created."},
          "400": {"$ref": "#/components/responses/BadRequest"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/auth/local/users/{user_id}/password": {
      "post": {
        "tags": ["auth"],
        "summary": "Reset a local identity password",
        "description": "All-scopes admin route that revokes active local credentials, stores a new bcrypt password hash, and clears lockout state.",
        "operationId": "resetLocalIdentityPassword",
        "parameters": [{"name": "user_id", "in": "path", "required": true, "schema": {"type": "string"}}],
        "requestBody": {
          "required": true,
          "content": {"application/json": {"schema": {"type": "object", "required": ["password"], "properties": {"password": {"type": "string", "format": "password"}}}}}
        },
        "responses": {
          "204": {"description": "Password reset."},
          "400": {"$ref": "#/components/responses/BadRequest"},
          "403": {"$ref": "#/components/responses/Forbidden"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/auth/local/password/rotate": {
      "post": {
        "tags": ["auth"],
        "summary": "Self-service local identity password rotation",
        "description": "Public pre-session route (issue #4976). Re-proves the caller's current_password (and recovery_code, when the account has an active MFA factor) instead of relying on an existing session, then stores a new bcrypt password hash and clears must_change_password. This is the only way a must_change_password=true credential -- the ESHU_ADMIN_USERNAME/PASSWORD[_FILE]-seeded bootstrap admin -- can ever obtain a session; any local user may also use it to voluntarily rotate their own password. Returns the same LocalIdentitySessionResponse shape as login: mfa_required (202) when the account has an active MFA factor and no recovery_code was submitted, locked (423) after too many failed attempts, or invalid (401) for a wrong current_password.",
        "operationId": "rotateLocalIdentityPassword",
        "requestBody": {
          "required": true,
          "content": {"application/json": {"schema": {"$ref": "#/components/schemas/LocalIdentityPasswordRotationRequest"}}}
        },
        "responses": {
          "200": {
            "description": "Password rotated; browser session issued.",
            "content": {"application/json": {"schema": {"$ref": "#/components/schemas/LocalIdentitySessionResponse"}}}
          },
          "202": {
            "description": "Credential proof accepted but more proof (MFA recovery code) or another rotation is still required.",
            "content": {"application/json": {"schema": {"$ref": "#/components/schemas/LocalIdentitySessionResponse"}}}
          },
          "400": {"$ref": "#/components/responses/BadRequest"},
          "401": {"description": "Wrong current_password or unknown login_id."},
          "423": {"description": "Local login is temporarily locked after repeated failed proof attempts."},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/auth/local/users/{user_id}/mfa-reset": {
      "post": {
        "tags": ["auth"],
        "summary": "Reset local identity MFA recovery material",
        "description": "All-scopes admin route that revokes active MFA factors and recovery codes, then stores replacement recovery-code hashes.",
        "operationId": "resetLocalIdentityMFA",
        "parameters": [{"name": "user_id", "in": "path", "required": true, "schema": {"type": "string"}}],
        "requestBody": {
          "required": true,
          "content": {"application/json": {"schema": {"type": "object", "required": ["recovery_codes"], "properties": {"mfa_factor_kind": {"type": "string"}, "mfa_credential_handle": {"type": "string"}, "recovery_codes": {"type": "array", "items": {"type": "string"}}}}}}
        },
        "responses": {
          "204": {"description": "MFA material reset."},
          "400": {"$ref": "#/components/responses/BadRequest"},
          "403": {"$ref": "#/components/responses/Forbidden"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/auth/local/users/{user_id}/disable": {
      "post": {
        "tags": ["auth"],
        "summary": "Disable a local identity",
        "description": "All-scopes admin route that disables a user and revokes active local credentials, MFA factors, and browser sessions.",
        "operationId": "disableLocalIdentity",
        "parameters": [{"name": "user_id", "in": "path", "required": true, "schema": {"type": "string"}}],
        "responses": {
          "204": {"description": "Local identity disabled."},
          "400": {"$ref": "#/components/responses/BadRequest"},
          "403": {"$ref": "#/components/responses/Forbidden"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/auth/local/break-glass": {
      "post": {
        "tags": ["auth"],
        "summary": "Enable time-boxed local identity break-glass recovery",
        "description": "Operator-controlled break-glass enablement. This route is not public, requires the shared operator bearer token, is disabled by default when no active window exists, stores only a break-glass code hash, and emits governance audit events for allowed or denied enablement.",
        "operationId": "enableLocalIdentityBreakGlass",
        "requestBody": {
          "required": true,
          "content": {"application/json": {"schema": {"$ref": "#/components/schemas/LocalIdentityBreakGlassRequest"}}}
        },
        "responses": {
          "201": {"description": "Break-glass window enabled. The break_glass_code is returned once and only the hash is persisted."},
          "400": {"$ref": "#/components/responses/BadRequest"},
          "403": {"$ref": "#/components/responses/Forbidden"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
    "/api/v0/auth/local/break-glass/session": {
      "post": {
        "tags": ["auth"],
        "summary": "Create a browser session from break-glass recovery",
        "description": "Public recovery route that succeeds only when an operator-enabled break-glass window is active, unexpired, and unconsumed. The submitted break-glass code is hashed before lookup and successful recovery emits a governance audit event.",
        "operationId": "createLocalIdentityBreakGlassSession",
        "requestBody": {
          "required": true,
          "content": {"application/json": {"schema": {"type": "object", "required": ["break_glass_code"], "properties": {"break_glass_code": {"type": "string", "format": "password"}}}}}
        },
        "responses": {
          "200": {"description": "Break-glass browser session created.", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/LocalIdentitySessionResponse"}}}},
          "401": {"$ref": "#/components/responses/Unauthorized"},
          "503": {"$ref": "#/components/responses/ServiceUnavailable"}
        }
      }
    },
`
