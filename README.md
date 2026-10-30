# Career Assessment 107 — Go edition

A traceable, versioned, evidence-aware career-information system, implemented
in **Go** and shipped as an **Android app (APK)**.

```
0_User  →  1:3_Infrastructure  →  Information  →  0_User
x_i → (A_i, E_i, C_i) | U_i        // observation → 1:3 representation + uncertainty
```

The user is the origin of intent and the decision authority. The system
captures an answer, turns it into a traceable (Assessment, Evidence, Context)
representation with explicit Uncertainty, and maps it onto pathway
*information*. It never produces a career score, a ranking, or a decision.

This repository is a port of the original TypeScript implementation
(`CareerAssesment-main`). The career semantics, identities, and research
results are reproduced exactly — see [Verification](#verification).

## What is in this repository

| Path | Contents |
| --- | --- |
| `domain/` | 107 semantics: signal, assessment, evidence, context, uncertainty, representation, transformation, pathway, invariants, versioning, records, capability contracts. No infrastructure, no UI. |
| `engine/` | Infrastructure: S0–S4 pipeline, capability registry and authorization, execution records, artifacts, persistence seam, Cloud107 adapter, model boundary, audit log. No career semantics. |
| `research/` | The Research LOOP: four 1:3 verification experiments, the deterministic fixture, the report generator, and the committed results. |
| `api/` | HTTP API over the runtime, session persistence (journal + replay), and the embedded web UI. |
| `mobile/` | The gomobile binding the Android app calls (`Mobile.start(dataDir)`). |
| `web/` | React + TypeScript UI: the mobile app (`src/app/`) and the technical view (`src/technical/`, `src/screens/`). It renders what the Go API returns; it holds no domain logic. |
| `android/` | Android app: a WebView shell around the Go runtime. |
| `cmd/career107` | Desktop/server binary: serves the API and the UI. |
| `cmd/research` | Runs or checks the research experiments. |
| `scripts/build-apk.sh` | One-command build: tests → UI → Go AAR → APKs in `dist/`. |
| `docs/` | Architecture, API, build, testing, and porting notes. |

## Quick start

```bash
# Run everything in the browser (http://127.0.0.1:8080)
cd web && npm ci && npm run build && cd ..
go run ./cmd/career107 -data ./data

# Tests (all Go packages, including parity with the original results)
go test ./...

# Research experiments
go run ./cmd/research check      # re-run and compare with research/results/
go run ./cmd/research run        # re-run and rewrite research/results/

# Build the Android app (debug + release APKs into dist/)
scripts/build-apk.sh
```

See [docs/BUILD.md](docs/BUILD.md) for toolchain requirements and installing
the APK on a phone.

## How the app works

The Go runtime runs **inside the app process**. On start, `MainActivity`
calls `Mobile.start(filesDir)`; Go opens an HTTP server on `127.0.0.1` (random
port), restores the saved session, and returns the URL; the WebView loads it.
All assessment logic, validation, and storage happen in Go. Nothing is sent
off the device, and the server is not reachable from the network.

```
┌──────────── Android app process ─────────────┐
│ MainActivity (Java)                           │
│   └─ WebView ── http://127.0.0.1:<port>/ ──┐  │
│                                            ▼  │
│ libgojni. so (Go)                              │
│   api ── session journal (files/session.json) │
│    └─ engine (S0–S4) ── domain (1:3)          │
└───────────────────────────────────────────────┘
```

## The app

The interface follows the client's design ("Career Intelligence — International
University", Academic Prestige design system: Source Serif 4 / Source Sans 3,
navy on a pale-blue surface, ledger cards, Material Symbols).

- **Candidate registration. **** Name, current situation and career stage, stored on the device; the situation is recorded as context with every answer.
- **Home Overview. **** Hero with benchmark tiles, the live battery card, the top career pathways, and the dossier snapshot.
- **Assessment Scope (Syllabus).** The 19 items in five modules, with progress; any item can be opened.
- **Live Test (Assess).** One item at a time: module, section completion, the item formulation,n and the 1–5 candidate response matrix (or choices / free text), Review Previous, and Save & Continue. Each answer is captured (S0) and decomposed into `(A, E, C) | U` (S1→S4) in one step.
- **Results Dossier. **** Candidate card with the primary alignment archetype and top self-rated strengths, Download Dossier (PDF), and every career pathway with its "% Matched".
- **Pathway detail.** Requirement ledger (met / not met / mixed/open, the user's response and the reasoning), entry routes, trade-offs, reflection questions, and the user's own decision: Pursue, Explore, Later, Not for me.
- **Student Portal.** Candidate details, decision record, assessment record, start over, and **Technical Details** — the original inspection screens (identities, A/E/C/U facets, research, architecture).

"% Matched" is the share of a pathway's requirements that the user's answers
support. It is computed in the UI for presentation only; the engine still
records requirement statuses, never a score (see docs/PORTING-NOTES.md).

Android's Back button steps back inside the app and leaves it from the home screen.

## What the system does

1. **S0 · Capture.** An answer becomes an immutable observation with a stable identity, an evidence record (source, method, time, quality), and explicit uncertainty.
2. **S1 · Represent.** A capability decomposes it into `(A, E, C)` with cross-cutting `U`; `A.id = E.id = C.id = SignalId`.
3. **S2 · Plan.** Capability, transformation version, inputs, limits, and the authorization decision are fixed before anything runs.
4. **S3 · Materialise.** The payload is stored as a digest-verified artifact and handed to the execution substrate with hard limits.
5. **S4 · Execute.** Execution record, artifacts, and transformation record are written; declared losses are recorded.
6. **P → 0.** Pathways are presented as alternatives with evidence, requirements, trade-offs, and open questions. The user's review is recorded as theirs.

## What the system refuses to do

- No universal career score, ranking, or "best career". Forbidden field names are rejected structurally.
- No personality diagnosis: the shipped instrument is marked `validated: false`.
- No model output used as evidence; model-dependent operations require full provider/model/instruction metadata.
- No decision by the system: only an actor of kind `user` can set a pathway's review state.
- No silent history rewrite: records are append-only, and digests are chained.

## Verification

- `go test ./...` — domain, engine, API, and research tests.
- `research/parity_test.go` re-runs the four experiments and compares every
  measurement, and the deterministic signal identities, against
  `research/results/experiments.json` and `experiments.md` produced by the
  original TypeScript implementation. They match exactly.

## License

GPL-3.0-or-later (see `LICENSE`), as the original.

