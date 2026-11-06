// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package openapi assembles the dest-served OpenAPI 3.0 document from the
// ported family path fragments under openapi/paths/ and the minimal
// shared components block. Only families whose fragments have landed are
// served natively (sign-in-policy first, per satellite-design Decision
// 5(c)); every other family is still relayed from core until its lane
// ports its fragment. The concatenation order is the compatibility
// surface the destination owns.
package openapi
