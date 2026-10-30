// Command eshu-api serves the eshu-api HTTP surface: the versioned
// routing, auth, v1 alias, and deprecation surface over health and
// readiness, and over the core origin through byte-for-byte dispatch of
// the authenticated /api/* subtree. It takes no core handles and makes
// no core calls except the HTTP dispatch to the configured origin.
package main
