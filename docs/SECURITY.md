# Security and trust boundaries

## What is enforced, and where

| Control | Implementation |
| --- | --- |
| Schema validation at every boundary | explicit validators in `domain/` and `engine/`; runtime re-validates payloads and re-runs invariants before and after execution |
| Authenticated user boundary | Every record names its actor; decision and correction records reject non-`user` actors |
| Authorization before execution | `AuthorizationService.evaluate` decides before S2; denials are recorded with a machine-readable reason |
| Capability allowlisting | `CAPABILITY_CONTRACTS` is the only source of capability ids; unregistered and non-allowlisted capabilities are refused by the registry and the authorization service |
| Immutable audit references | Chained digests in `domain/records.go`; append-only persistence contract; `engine.AuditLog` (SHA-256) writes a chained JSON Lines trail with SHA-256 |
| Model/version metadata | `INV.MODEL.AUDITABLE` — a model-dependent transformation without full metadata is refused before it can be recorded |
| Artifact integrity | Digests computed at write time, re-verified after execution; verification failure fails the invocation |
| Input size limits | `MAX_INPUT_BYTES` at capture; `maxInputBytes` per capability contract and per grant |
| Safe handling of model output | `engine/model.go`: control characters removed, scheme-bearing links neutralised, length bounded, injection-style content flagged and retained for review; proposals can never be evidence |
| No arbitrary execution | Assessment input is stored as text; nothing evaluates, imports, or interpolates it. Capability payloads are declarative and digest-verified |

## Digest strength is stated, not implied

The domain ships a deterministic, portable hasher (FNV-1a, 64-bit) for
reproducible identities and says so: every digest carries
`algorithm`, `strength`, `value`, and `covers`. Audit trails use the Node entry
point's SHA-256 hasher, and the recorded strength changes accordingly. A weak
integrity check can therefore never masquerade as a strong one.

## Threat model notes

- **Model output is untrusted input.** It is sanitised, bounded, flagged, stored as a proposal, and excluded from evidence. It cannot change a status, a score, or a record by itself.
- **The user is the authority, not a trusted input source.** A user can be inconsistent or mistaken; the architecture keeps inconsistent observations rather than resolving them silently, and reports the disagreement in the requirement status (`conflicting`).
- **Third-party subjects.** `SubjectSchema` distinguishes `self` from `third-party` and gives third-party subjects a `consentRef` field. Consent enforcement at the boundary is **not implemented** — it is listed in the roadmap and in the unverified section of the report.
- **No authentication in this build.** The web application runs entirely client-side with a single local subject. Authentication, session binding, and server-side authorization are deployment work; the contracts they must satisfy (actor-bound grants, subject binding, budgets) already exist.
- **Privacy field guard is structural, not legal.** It prevents the architecture from *storing* certain characteristics; it does not constitute a compliance determination.

## What this system must never do

- Store a universal career score or rank a person.
- Treat model output as evidence, or let a model-dependent operation run without auditable metadata.
- Let a system, capability, or model actor set a user decision.
- Overwrite historical state, or accept an unverified artifact as intact.
- Execute assessment input, or allow network access during assessment execution.
- Move career semantics into the infrastructure layer.
