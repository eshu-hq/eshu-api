# Destination telemetry (Refs `eshu-hq/eshu#9669`, part of `eshu-hq/eshu#6707`)

Satellite Leaf E. The destination answers the operator question
"which family, native or relay, denied or 503, core slow or dest slow"
from its own signals alone: per-family request and dispatch metrics on
`GET /metrics`, plus one structured log line per dispatch request keyed
on the correlation ID the auth gate echoes-or-mints.

## Signals

`GET /metrics` is public (like the probes) and serves Prometheus text
exposition. Every label is a closed vocabulary: family comes from the
bounded table in `api/family.go`, never from raw path params, so series
cardinality is bounded by (families x classes x lanes x outcomes).

| Instrument | Labels | What it tells the operator |
| --- | --- | --- |
| `eshu_api_requests_total` | `family`, `class` (`2xx`–`5xx`, `unknown`) | Which family, and its status mix (gate 401s count here too). |
| `eshu_api_request_duration_seconds` | `family` | Dest-side latency per family (end to end). |
| `eshu_api_dispatch_total` | `family`, `via` (`native`, `relay`), `outcome` | Which lane served it, and what happened: `core_ok`, `core_503`, `core_retryable_503`, `local_503`, `denied`. |
| `eshu_api_core_latency_seconds` | `family`, `via` | Core-side latency. Compare with the request duration to split core-slow from dest-slow. |
| `eshu_api_authorizer_total` | `family`, `decision` (`allow`, `deny`) | Family authorizer judgments. The deny branch is unreachable in production today (shared key maps to AllScopes), so a `deny` here is the canary for the scoped-resolver day. |
| `eshu_api_audit_drops_total`, `eshu_api_audit_dropped_events_total` | none | Swallowed audit-append failures observed through the Leaf A drop-observer hook. Dest emits no audits today, so these rest at zero until the audit-append port lands. |

Outcome semantics follow the read-transport error rules: a `local_503`
is synthesized by dest (unconfigured origin, unreachable core,
unbuildable request) and never carries `Retry-After`; a relayed core
503 keeps the `Retry-After` the core sent only when the verdict was
marked retryable (`core_retryable_503` vs `core_503`).

## Logs

One `api request` line per dispatch request at info level with
`method`, `family`, `via`, `status`, `status_class`, `outcome`,
`duration_ms`, `core_latency_ms`, `retry_after`, and `correlation_id`.
The credential never appears. The correlation ID is the caller's
`X-Correlation-ID` (or `X-Request-ID`) when supplied, minted otherwise,
and the dispatch response echoes it back on the `X-Correlation-ID`
header; probes never carry it. Trace context (`traceparent`,
`tracestate`) crosses to the core untouched — dest forwards, never
interprets — and the correlation ID is guaranteed upstream. Dest
invents no truth levels or error codes: the only synthetic error stays
the versioned `backend_unavailable` 503.

## Family table

Prefixes live in `api/family.go`, longest match wins, everything else
is `other`. Route evidence: `contracts/upstream/*-wire-shapes.txt`
for the mutation families, the read-transport Native-now wave for the
read families. The native lane carries the tier-C dispatch families
plus the D2 Native-now wave (capabilities, CLI support, ask, service
intelligence, auth providers, sign-in-policy reads, provider-config
reads, doc UI, served spec) plus the D3 admin/identity/recovery reads
wave (admin query POSTs, admin identity GETs, governance-audit GETs,
terminal-failure query POSTs) plus the D7 admin-four tier-C clones
(backfill, dead-letter, reindex, skip POSTs, family `admin`,
`via="native"`) plus the D8 session-resolver tier-C clones
(browser-session lifecycle `browser_session`, session-list
`session_list`, profile `profile`, local-identity/token/TOTP
`local_identity`, and the invitations create/accept routes that keep
the pre-existing `admin_identity` prefix — all `via="native"`) plus
the D9 OIDC-login tier-C clones (login start and callback GETs,
family `oidc_login`, `via="native"`) plus the D9 GitHub-login tier-C
clones (login start and callback GETs, family `github_login`,
`via="native"`) plus the D9 SAML-login tier-C clones (metadata and
login-start GETs plus the ACS POST, family `saml_login`,
`via="native"`) plus the setup carve-out (family `setup`,
`via="native"`: the state GET terminates via the SetupNeeded port
while the three credential POSTs terminate as writers filing
native-lane outcomes (dest-local local_503, forwarded legs via the
shared path) — same lane and outcome vocabulary, extended transport,
no renames);
every other family crosses the relay lane until its
native route lands, and the `via` label tells the two apart. Wave-gated routes that relay explicitly for unresolvable
identity classes file on `via="relay"` with their wave family, so the
lane always names who served the bytes.
