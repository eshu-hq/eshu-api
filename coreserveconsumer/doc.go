// Command coreserveconsumer is the N1 (issue #446) external-consumer proof
// for the shared auth serving library required by ADR-0002 section 2.
//
// It is a separate Go module so the main eshu-api module never imports the
// seam transitively: the proof imports ONLY the versioned shared library
// (github.com/eshu-hq/eshu/go/coreserve) and serves /readyz plus one auth
// read against fixture stores with zero core-origin HTTP calls routed via
// http.DefaultTransport (custom Transports or direct dials bypass the
// test guard and are out of this proof's scope).
//
// Status: LANDED. The go/coreserve library is tagged v0.1.0 in
// eshu-hq/eshu, this module requires exactly
// github.com/eshu-hq/eshu/go/coreserve v0.1.0, and the proof command
// `cd coreserveconsumer && go vet ./... && go test -race ./... -count=1`
// passes locally and in the coreserve-consumer CI job. No replace directive
// is present in the committed go.mod. Production paths (cmd/eshu-api,
// api/auth.go, all api/*_terminate.go leaves) stay untouched until P5.
package coreserveconsumer
