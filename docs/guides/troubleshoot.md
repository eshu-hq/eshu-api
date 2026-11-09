# Troubleshoot eshu-api

Work through these in order — they're the failures you'll actually
meet, roughly in the order you'll meet them.

## 1. It won't start: `--core-base-url is required`

You see `eshu-api: --core-base-url is required` and exit code `1`.
You didn't pass the flag. It's the only required one. Pass your core's
address and retry.

An unknown flag fails the run the same way, instead of booting with a
half-parsed config. Check your spelling against
[Reference: configuration](../reference/configuration.md).

## 2. It's up, but every gated `/api/*` call is `401`

Usually `ESHU_API_KEY` is unset (empty fails closed) or your callers
send a different key than the one you set.

Remember the public setup, login, and docs routes won't `401`, so test
with a gated route like `/api/v0/openapi.json` before you conclude
auth is broken — see [Auth setup](auth-setup.md).

## 3. `/api/*` returns `503 backend_unavailable`

Check the `Retry-After` header; it tells you whose `503` this is:

- **No `Retry-After`** — that's our local answer when the core can't
  be reached or the origin doesn't parse. Check `--core-base-url`
  first, then check the core itself.
- **Carries `Retry-After`** — that came from the core and means
  something different.

[Telemetry](../telemetry.md) shows how the metrics tell the two apart
and split "core is slow" from "we are slow".

## 4. The port is already taken

The listener error comes back after the startup log line, and the
process exits. Free the port or pick another `--addr`.

## Still stuck?

- Confirm what the process thinks it's doing: `curl /health` shows the
  configured core origin and served contract versions.
- Follow one request in the JSON logs by its correlation ID.
- Re-check the picture in [Diagrams](../diagrams/README.md) — most
  surprises are a request landing on the wrong side of the gate.
