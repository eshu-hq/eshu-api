// Package api owns the eshu-api HTTP surface: routing, the shared-key
// auth gate, the v1 alias rewrite, the v0 deprecation headers, and the
// byte-for-byte dispatch of the authenticated /api/* subtree to the
// configured core origin. The D2 Leaf B Native-now wave terminates 18
// routes natively ahead of that relay (see leafb_wave.go and
// contracts/leafb-native-now.md); every other family still crosses the
// relay lane. It also owns the Leaf E telemetry registry:
// per-family request, dispatch, authorizer, and audit-drop signals on
// the public /metrics exposition plus the structured per-request log
// (see docs/telemetry.md).
//
// Every behavior here is ported from the versioned core contracts pinned
// under contracts/ (CoreReadContractsVersion, currently v0) and the
// in-tree surface it replaces (go/cmd/api in eshu). This package takes
// no core implementation handles and no shared-table shortcuts: core
// truth arrives only over HTTP to the configured origin — relayed
// through dispatch for /api/*, terminated natively on the versioned
// admission ports — and durable mutations persist only through
// versioned core admission contracts. internal/readclient offers the
// same timeout and byte budget for versioned read-contract use, but no
// production lane calls it yet: the relay carries its own bounded
// client pinned to the same bound by test.
package api
