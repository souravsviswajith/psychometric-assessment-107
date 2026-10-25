# Architecture

## 1. The boundary

```
0 = User                     origin and decision authority
1:3 = Infrastructure         the representation the system operates on
```

The loop is:

```
0 → Observation → 1:3 → Transformation → PathwayInformation → 0
```

The system never performs `0 → CareerDecision`. The decision stays outside its
authority, and the code enforces that: a pathway's review state can only be set
by an actor of kind `user` (`INV.PATHWAY.USER-AUTHORITY`).

## 2. The root model

```
I_t → S0 → S1 → S2 → S3 → S4 → O_t → I_{t+1}
```

| Stage | Meaning | Implemented by |
| --- | --- | --- |
| `I_t` | user intent / input | `SignalInput` |
| `S0` | captured semantic input | `domain/signal.go` (`CaptureSignal`), runtime `CareerRuntime.CaptureSignal` |
| `S1` | validated 1:3 representation | `domain/representation.go` (`DecomposeSignal`) |
| `S2` | constrained execution plan | `engine/invocation.go` (`InvokeThroughPipeline`) |
| `S3` | platform/runtime representation | `RuntimeInvocation` + artifact store |
| `S4` | executed state / artifact | `ExecutionRecord`, `TransformationRecord`, artifacts |
| `O_t` | observable result | rendered representation, pathway set, execution record |
| `I_{t+1}` | subsequent user decision | `UserDecisionRecord`, `CorrectionRecord` |

The broader pipeline —

```
Representation → Constraint → Transformation → Allocation → Execution → State → Observation → Feedback
```

— maps onto those stages: representation (`S1`), constraint (`S2` limits and
authorization), transformation (capability handlers), allocation (capability +
grants), execution (`S3`/`S4`), state (records), observation (renderers,
projections), feedback (user decisions, corrections, re-mapping).

## 3. 1:3

```
x_i → (A_i, E_i, C_i) | U_i
```

- **A — Assessment.** What was observed, measured, answered, or assessed. Carries the instrument and item version, the exact prompt, the interpreted response with the policy that produced it, and the constructs the item bears on.
- **E — Evidence.** Why the observation exists and what supports its provenance: `E_i = (P_i, M_i, T_i, Q_i)` — provenance, measurement method, temporal information, quality metadata. Exactly one primary record per observation; derived records must declare lineage.
- **C — Career context.** The context in which the observation may legitimately be relevant: declared by the user, session, instrument, environmental, or mixed.
- **U — Uncertainty.** Cross-cutting metadata *over* `(A, E, C)`, never a fourth dimension. Sixteen registered codes, each with a source, kind (epistemic/aleatoric), affected dimensions, a level with a registry floor, and a mitigation.

Identity invariant:

```
SignalId(A_i) = SignalId(E_i) = SignalId(C_i)
```

Identity is minted once, at the observation, and carried. A/E/C records that
cannot be traced back to that single identity are rejected at parse time and by
`INV.IDENTITY.CARRIED`.

## 4. What is deliberately absent

`careerScore: number` does not exist, and a structural guard rejects it:
`scanForScalarCareerQuantities` walks any representation, record, pathway set, or
persistence document and raises `INV.NO-SCALAR-CAREER-QUANTITY` for forbidden
field names (`careerScore`, `fitScore`, `matchScore`, `rank`, `bestCareer`, …).

The relationship the architecture asserts instead:

```
1 observation ⇒ 3 representations ⇒ validated transformations ⇒ explainable pathway information
```

## 5. 107 / Cloud107 boundary

- **107 owns semantics.** Domain contracts, transformations, assessment/evidence/pathway semantics, invariants, versioning, records. `domain/` has no dependency on execution, scheduling, persistence technology, or UI.
- **Cloud107 owns infrastructure.** Workload execution, scheduling, capability invocation, artifact handling, execution state. `engine/cloud107.go` defines the interface; the development deployment uses an in-process adapter, and a production deployment substitutes a queue/worker adapter without changing the domain.

Career semantics never cross that boundary. `scanPayloadForCareerSemantics`
refuses to plan a payload carrying scoring rules or conclusions, and the
Cloud107 unit test asserts it.

## 6. Capability boundary

```
Capability → Authorization → S1 → S2 → Execution → S4
```

Registered capabilities (allowlisted in `domain/capability.go`):

- `career.assessment.decompose-1to3` — deterministic, `representation.decompose` + `records.write`
- `career.assessment.map-pathways` — deterministic, `pathway.map` + `records.write`
- `career.assessment.propose-questions` — model-dependent, `proposal.generate`
- `career.assessment.explain-representation` — model-dependent, `representation.explain`

Authorization is decided before execution and recorded: allowlist, actor binding,
grant coverage, subject binding, scope coverage, expiry, budget, and input size.
Every denial names the rule that produced it.

## 7. Runtime validation

Data arriving as JSON is not trusted because it type-checks; the 107 semantics are
enforced at runtime. Every boundary has an explicit validator (`Validate*` in `domain/` and `engine/`) (`SignalInput`, `Signal`, `Representation1to3`, `EvidenceBundle`,
`PathwaySet`, `CapabilityRequest`, `ExecutionPlan`, `RuntimeInvocation`,
`ExecutionRecord`, `TransformationRecord`, `ProposalRecord`), and invalid
representations are rejected *before* transformation or execution.

## 8. Historical integrity

- Observations are immutable; a corrected answer is a new signal.
- Representations are never re-created for the same signal; a second decomposition is refused.
- Revisions are append-only with chained digests (`domain/records.go`); overwriting a stored revision is refused, and `verifyChain` detects tampering.
- Corrections and decisions are new records referencing the original.
- Remapping pathways preserves the user's review states.

## 9. The web and mobile projection

The UI renders domain state and nothing else: `UI ≠ Truth`. It shows the
`(A, E, C) | U` facets, the evidence, the uncertainty, the pathway requirements
with their citations, the invariant registry, and the experiments. It never shows
a score, a rank, or a recommendation.

The UI (`web/`, React) has two projections of the same state: the mobile app
(`src/app/`, built to the client's design: registration, home overview,
assessment scope, live test, results dossier, pathway detail, student portal)
and the technical view (`src/technical/`, `src/screens/`: the original
inspection screens). The app shows a "% Matched" figure per pathway and orders
pathways by it, as the client's design does; the figure is the share of
requirements whose status is `supported`, computed in the UI and never stored.

Neither projection holds domain logic. It reads `/api/catalog` and
`/api/session` and posts actions; the Go runtime validates, authorizes, executes,
and returns the new state (see [API.md](API.md)). Even representation rendering
(`domain.RenderRepresentation`) happens in Go, so what a reviewer sees is the
same rendering the research experiments measure.

On Android, the Go runtime is compiled into the app (`mobile/`, via gomobile)
and serves the API and the embedded UI on `127.0.0.1` inside the app process.
`android/` is a thin WebView shell: it starts the runtime with the app's private
files directory and loads the returned URL. No network service is involved.

```
MainActivity ─ Mobile.start(filesDir) ─▶ api.NewSession ─▶ engine.New ─▶ domain
     │                                        │
     └── WebView ── http://127.0.0.1:<port>/ ─┘  (UI + /api/*)
```

## 10. Persistence

`CareerPersistence` (`engine/cloud107.go`) is the storage seam (§16). The default
implementation is in-memory and append-only: writing a different payload under an
existing id is refused. A durable implementation stores the same documents with
the same append-only semantics; no domain code changes.

Sessions survive an app restart through the API's journal (`api/session.go`):
every action is appended to `session.json` with the clock readings it consumed,
and the journal is replayed on start. Because identities and digests are derived
from content and time, replay reconstructs exactly the same state — the same
property that makes the research fixtures reproducible.

## 11. Concurrency

The runtime serialises operations with a mutex, and the API session applies one
action at a time, so concurrent requests from the UI cannot interleave inside the
S0–S4 pipeline. The research experiments build their own runtimes and never touch
the user's session.
