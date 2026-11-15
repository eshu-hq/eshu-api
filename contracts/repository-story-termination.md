# Repository story termination (W5 satellite record)

Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#97. This lane
terminates the W5f story single natively ahead of the generic relay,
superseding the retired D4 relay leg.

## Pin

`RepositoryStoryReadAdmission v0` (15 re-declared methods), recorded
satellite-side in contracts/repository-story-read-admission.md plus the
vendored upstream copies
(contracts/upstream/repository_story_read_admission.go,
contracts/upstream/repository_story_read_admission_contract_test.go,
byte-identical at core `76240fda9f`) and the wire-shapes excerpt
(contracts/upstream/repository-story-read-wire-shapes.txt).
Enforcement: `scripts/check-parity.sh` (`SRC_SHA_W5ENTITYIMPACT`
block) plus the core-side frozen method-set test, satisfiability
witness, seeded re-signed rejection, and production-store mirror.

## Terminated (1)

| Route | Family | Gate |
|---|---|---|
| `GET /api/v0/repositories/{repo_id}/story` | repositories | storyReadGate |

The supersession: the story GET rode the explicit D4 relay leg
(d4RelayLegs, carried by TestD4StoryRelayLegOnGenericProxy) because
its spec entry waited on the schema.EvidenceBoundaries helper port.
This lane retires that leg — d4RelayLegs is empty, the relay test is
replaced by a supersession note pointing here, and the route
terminates natively behind the story pin. The spec fragment still
waits on the helper port, so the absence stays disclosed (rewritten
from relay to terminated-but-disclosed in api/spec_scope_test.go and
carried by openapi/spec_w5_test.go). Minting a whole-Repositories pin
to cover this route would retroactively widen D4, which stays
forbidden; the impact packet single rides its own pin and neither
route may cite the other's pin.

The wave gate answers every credential shape per the entity/impact
lane rule; the family authorizer (authorizeStorySharedKey) allows only
the shared-key (AllScopes) identity; the only dest-synthesized errors
are the gate 401, the future-resolver family 403, and the permanent
local 503 (never Retry-After).

## Proofs

- Frozen wave: TestStoryWaveFrozen (story_conformance_test.go).
- Ahead of relay: TestStoryWaveAheadOfRelay (native lane filed;
  upstream path intact) — this is the green side of the retired relay
  test's RED.
- Byte parity: TestStoryByteParity, TestStoryScopedEmptyGrantRelay
  (core 404 refusal untouched; relay lane filed).
- Authorizer + deny shape: TestStoryDenyBranchShapePinned, relay-
  explicit and junk-cookie suites, empty-key fail-closed, nil-core 503.
- Latency: BenchmarkStoryTerminatedRead, bounded by
  scripts/measure-w5-latency.sh; numbers in docs/w5-latency.md.
- Spec bijection: the mounted story route has its disclosed absence
  (no served entry, no missing mount) — the bijection closes at this
  record time, not in the pin.

## Rollback

Remove the `s.registerStoryWave(mux)` call in routes() (or this
lane's files): the story GET returns to the generic relay behind
RequireAuth. The spec absence stays disclosed either way. No data
migrates and no other family moves.
