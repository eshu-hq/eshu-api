# One request's story

Here's a single call from doorstep to answer. The drawn version is in
[Diagrams: what happens to one request](../diagrams/README.md).

## The trip

1. You send a request, say `GET /api/v1/repositories` with a Bearer
   token.
2. We tidy up the version: `/api/v1/*` is rewritten to `/api/v0/*`
   before dispatch.
3. We check your token when the route asks for one. Missing or wrong
   on a gated route? You get a local `401` and the trip ends.
4. Otherwise we get out of the way: the same method, path, headers,
   and body go to the core, and the core's status, headers, and body
   come back to you untouched.
5. If we can't reach the core at all, you get a local
   `503 backend_unavailable` with no `Retry-After` header. That
   missing header is deliberate — it tells you the failure is on our
   side of the fence, not a "try again in five seconds" from the core.

Details worth knowing:

- Replies for `/api/v0/*` carry `Deprecation` and `Sunset` headers.
  Replies for `/api/v1/*` don't, because the deprecation headers are
  added before the rewrite runs.
- Your token crosses to the core only in the `Authorization` header.
  It never shows up in logs or health output.
- `GET /api/v0/openapi.json` is served from compiled constants in this
  repo, so it works even with no core origin configured. It still
  needs a token (see
  [Reference: configuration](../reference/configuration.md)).

## What comes in

Three kinds of traffic arrive here:

- **Probes we answer ourselves:** `GET /health`, `GET /readyz`, and
  `GET /metrics`. No token, no core call. `/health` reports `status`,
  `service`, one version key per contract family, and the configured
  core origin. `/metrics` serves Prometheus text; [Telemetry](../telemetry.md)
  lists every series.
- **Gated API calls under `/api/*`:** most need a valid Bearer token
  and then dispatch to the core. Some write families are handled here
  first and written through a versioned core contract instead of
  relayed blindly. Those contracts live under `contracts/`.
- **Public `/api/*` routes:** the first-run setup wizard, local login
  and invitation acceptance, OIDC, GitHub, and SAML sign-in, plus the
  docs pages and login-picker data. They register outside the auth
  gate on purpose, and the core still makes every trust call.
  [Reference: configuration](../reference/configuration.md) names
  each one.

## What goes out

Most replies are the core's bytes exactly as sent. The exceptions are
ours: the probe JSON, the compiled OpenAPI document, the docs and
redoc shells, that `401`, and the local `503`. Errors come back in the
shape you asked for: ask for `application/eshu.envelope+json` and you
get the envelope error section, otherwise the plain `error_code`
shape. Either way you get a correlation ID, and each dispatch writes
one `api request` JSON log line with the method, family, status, and
that same ID.

One more thing, because it surprises people: if `ESHU_API_KEY` is
empty, the gated routes don't become open. They fail closed until you
set a key. The public routes above stay public either way — the full
story is in [Auth setup](../guides/auth-setup.md).
