# Invariants

Invariants are registered in `domain/invariants.go` as named,
executable checks with a statement in reviewer language. They are run by the test
suite, by the runtime before transformation, and by the research experiments, so
"the invariant holds" is a measurement rather than a claim.

| Id | Scope | Statement | Enforced by |
| --- | --- | --- | --- |
| `INV.IDENTITY.CARRIED` | representation | `A.id = E.id = C.id = SignalId`: identity is carried through the transformation, never re-minted. | schema refinement on `Representation1to3`; `checkRepresentation` |
| `INV.EVIDENCE.PRIMARY` | signal | Every observation has exactly one primary evidence record, and every derived record has lineage. | `EvidenceBundleSchema`, `createDerivedEvidence` |
| `INV.EVIDENCE.PROMOTED-CLAIM` | signal | A promoted claim carries a non-null evidence reference; an unsupported claim is refused. | `createPrimaryEvidence`, `checkSignal`, `assertProposalOnlyUse` |
| `INV.UNCERTAINTY.PRESERVED` | transformation | Uncertainty is preserved through transformations: notes are unioned, coverage weakens, declared losses require a note. | `mergeUncertainty`, `recordTransformation`, `checkTransformationRecord` |
| `INV.NO-SCALAR-CAREER-QUANTITY` | representation | No representation, record, pathway set, or document collapses `(A, E, C, U)` into a scalar career quantity. | `scanForScalarCareerQuantities` |
| `INV.PATHWAY.SUPPORT` | pathway | Every pathway candidate references the representations and transformations that support it, and a non-undetermined requirement cites evidence. | `mapToPathways`, `checkPathwaySet` |
| `INV.PATHWAY.NO-RANK` | pathway | Pathway sets carry no total ordering, no hidden score, and no single "best career". | `scanForScalarCareerQuantities` over the set |
| `INV.PATHWAY.USER-AUTHORITY` | pathway | Pathway review state is set by the user (0); the system records but never assigns a decision. | `setPathwayReview`, `createUserDecisionRecord`, runtime `setPathwayReview` |
| `INV.VERSION.NON-DESTRUCTIVE` | records | Historical representations are versioned, never silently mutated. | `appendRevision`, `attemptOverwrite`, `verifyChain`, persistence append-only contract |
| `INV.MODEL.AUDITABLE` | transformation | Model-dependent transformations record provider, model, model version, instruction version, input reference, output reference, uncertainty, and transformation version. | `recordTransformation`, `checkTransformationRecord` |
| `INV.INPUT.SIZE-LIMIT` | signal | Assessment input is bounded and treated as data; no assessment input is executed. | `MAX_INPUT_BYTES`, capture-time byte check, capability `maxInputBytes` |
| `INV.UI.NOT-TRUTH` | ui | The UI projects domain state; it does not define it. | Web projection renders only domain objects; no client-side career logic |

## Structural guards that back the invariants

- **Forbidden field guard** (`domain/fieldguard.go`): career context keys, uncertainty details, and other free-text fields are refused if they name characteristics outside the career-assessment boundary (for example ethnicity, religion, health status, immigration status).
- **Forbidden score guard** (`domain/invariants.go`, `ScanForScalarCareerQuantities`): field-name scan for scalar career quantities.
- **Boundary scan** (`engine/cloud107.go`, `ScanPayloadForCareerSemantics`): payloads carrying career semantics cannot cross into the infrastructure layer.
- **Model output handling** (`engine/model.go`): control characters removed, links neutralised, length bounded, injection-style content flagged and retained for review rather than obeyed; proposals can never be evidence.

## Why some invariants are structural rather than conventions

Conventions are invisible in a diff. A `careerScore` field, a re-minted identity,
or a silently discarded uncertainty note can look like harmless convenience code.
Each of those decisions is therefore encoded as a check that runs in CI and at the
boundary, so the failure mode is a rejection with a code, not a subtle drift.
