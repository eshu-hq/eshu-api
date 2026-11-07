// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package openapi assembles the dest-served OpenAPI 3.0 document from the
// ported family path fragments under openapi/paths/ and the minimal
// shared components block. Only families whose fragments have landed are
// served natively (sign-in-policy first, retirement second,
// terminal-failure third, admin-identity fourth, provider-config
// mutations fifth, admin-recovery sixth, suppression seventh, per
// satellite-design Decision 5(c), then the D7 admin-four tier-C clone
// lane (AdminD7), then the D2 Leaf B Native-now wave:
// capabilities, CLI support, ask, service intelligence, auth providers,
// provider-config reads, and the served-spec self entry); every
// other family is still
// relayed from core until its lane ports its fragment. The
// concatenation order is the compatibility surface the destination
// owns.
package openapi
