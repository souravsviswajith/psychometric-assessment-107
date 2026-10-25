# Porting notes: TypeScript → Go

## Where things went

| Original (TypeScript) | Go |
| --- | --- |
| `career-domain/src/core/*` | `domain/core.go`, `domain/fieldguard.go` |
| `career-domain/src/identity/*` | `domain/identity.go`, `domain/canonical.go` |
| `career-domain/src/assessment/*` | `domain/assessment.go`, `domain/instrument.go` |
| `career-domain/src/evidence/*` | `domain/evidence.go` |
| `career-domain/src/context/*` | `domain/context.go` |
| `career-domain/src/uncertainty/*` | `domain/uncertainty.go` |
| `career-domain/src/signal/*` | `domain/signal.go` |
| `career-domain/src/representation/*` | `domain/representation.go` |
| `career-domain/src/invariants/*` | `domain/invariants.go` |
| `career-domain/src/transformation/*`, `versioning/versions.ts` | `domain/transformation.go` |
| `career-domain/src/records/*`, `versioning/store.ts` | `domain/records.go` |
| `career-domain/src/pathway/*` | `domain/pathway.go`, `domain/catalogue.go` |
| `career-domain/src/capability/contracts.ts` | `domain/capability.go` |
| `career-runtime/src/execution/*` | `engine/types.go`, `engine/artifacts.go` |
| `career-runtime/src/capabilities/*`, `pipeline/scopes.ts` | `engine/capabilities.go`, `engine/handlers.go` |
| `career-runtime/src/cloud107/*`, `persistence/*`, `pipeline/trace.ts` | `engine/cloud107.go` |
| `career-runtime/src/model/*` | `engine/model.go` |
| `career-runtime/src/pipeline/invocation.ts`, `runtime.ts` | `engine/invocation.go`, `engine/runtime.go` |
| `career-runtime/src/node/audit-log.ts` | `engine/auditlog.go` |
| `research/src/*`, `research/scripts/*` | `research/*.go`, `cmd/research` |
| `apps/web` (with the runtime running in the browser) | `web/` (UI only) + `api/` + `cmd/career107` + `mobile/` + `android/` |
| `tests/*.test.ts` (vitest) | `*_test.go` |

## How fidelity was kept

- **Identities and digests are byte-compatible.** Canonical JSON follows
  `JSON.stringify` exactly (key order, string escaping, number formatting), FNV-1a
  64 is applied to the same UTF-8 bytes, and `MintID` reproduces the original's
  double canonicalisation. The same input yields the same `sig_…`, `r13_…`,
  `pths_…` identities in both implementations.
- **The clock is read the same number of times, in the same order,** so
  stepping-clock fixtures produce the same timestamps and therefore the same ids.
- **`toFixed` and `String(number)`** are reproduced (`domain.JSFixed`,
  `domain.JSNumber`), including tie rounding.
- **String lengths are counted in UTF-16 code units**, as the original limits
  were.
- The research parity test proves all of the above on the committed results.

## Deliberate differences

1. **Validation without Zod.** Each schema became an explicit `Validate*`
   function with the same constraints and the same custom messages. Generic
   messages ("Too small: expected string to have >=1 characters") follow Zod's
   wording but are not guaranteed identical.
2. **Synchronous runtime.** Promises became plain calls; the runtime is
   guarded by a mutex so the API can serve concurrent requests safely.
3. **Evidence builder pre-checks are enforced.** In the original,
   `createPrimaryEvidence` / `createDerivedEvidence` computed issues such as a
   whitespace-only source reference or `recordedAt` before `observedAt`, but
   returned success whenever the schema itself passed. The Go builders reject
   in those cases, as their documentation says they should. Normal captures are
   unaffected (parity holds).
4. **"Your decisions" count on the Pathways screen.** The original summed every
   counter of the decision summary after the first, which included
   `unreviewed`, so it always showed the number of candidates. It now counts
   explored + declined + pursuing.
5. **The UI no longer contains the runtime.** It calls the Go API. The input
   identity (`inputId`) and the subject are assigned by the server, not the UI.
6. **Sessions are saved.** The original kept everything in browser memory. The
   Go API journals actions to `session.json` and replays them on start.
   The stub model's `generatedAt` is wall-clock time (as in the original), so a
   replayed proposal shows a new `generatedAt`; its identity is unchanged.
7. **Panics only for programming errors.** Where the original called
   `Schema.parse` on values the domain built itself (and would throw), Go panics;
   capability handlers run behind a recover so a fault fails that execution
   instead of the process.
8. **Toolchain.** `golang.org/x/mobile` requires Go 1.26, so `go.mod` declares
   1.26; Go fetches that toolchain automatically.
9. **Presentation follows the client's design, including "% Matched".**
   The original UI never showed a number per pathway and kept pathways in
   catalogue order. The client's design shows "% Matched" badges, orders
   pathways by them, names a "primary alignment archetype", and shows strength
   bars. The app implements these as presentation over the engine's output:
   "% Matched" = requirements supported ÷ requirements; the archetype is the
   pathway with the highest share; strengths are the user's own highest
   self-ratings (shown as a percentage of the scale, not as population
   percentiles, which the instrument cannot provide). The engine itself is
   unchanged: it still records requirement statuses, rejects score fields in
   its records, and its invariants and research parity still hold. Claims in
   the design that the system cannot back (verified credentials, ISO
   certification, cohort sizes, percentiles) are replaced with real figures or
   neutral wording.
