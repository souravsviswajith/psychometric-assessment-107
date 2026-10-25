# Research position

## 1:3 is an architectural hypothesis, not an established law

The project's claim is architectural:

```
raw → (A, E, C) | U      preserves information that      raw → scalar      destroys
```

`A`, `E`, `C`, and `U` are not presented as a validated psychometric structure.
They are the representation the infrastructure is built on. Whether the
representation is *sufficient* for career assessment is an empirical question that
this repository does not answer; it provides the machinery to test it.

Two consequences are enforced in code:

1. The shipped instrument (`instrument.self-description@0.2.0`) is marked
   `validated: false` on every item, and the resulting uncertainty note
   (`U-INSTRUMENT-UNVALIDATED`, level `high`) travels with every representation
   derived from it and appears on every pathway.
2. `defaultPolicy.validatedInstruments` is empty. Adding an instrument to that
   list is an explicit act, and the evidence for it belongs in an instrument
   validation record, not in the code.

## The LOOP

```
Observe → Formalize → Transform → Verify → Execute → Compare → Measure → Refine → Repeat
```

The loop is executable: `go run ./cmd/research run` builds fixtures through the real
pipeline (capture → capability → decompose → capability → map), runs the four
experiments, writes `research/results/experiments.md`, and writes a machine-readable
`experiments.json`. The generated report is a record of a run that happened.

## Experiments

### A — Information preservation

Enumerate the facts a reviewer could want to recover from each raw observation
(instrument and item versions, exact prompt, interpreted response and the policy
that produced it, source, method, timestamps, quality metadata, context fields,
uncertainty codes, identities), then measure how many are recoverable from

- the `(A, E, C) | U` representation, and
- a scalar mean of response positions.

Recoverability is checked by set membership against the primitive values recorded
in the representation, so the measurement is a lookup rather than an assertion.
The scalar result is a fair representation of the compression the architecture
forbids: one aggregate value plus an observation count, with no provenance,
context, or uncertainty.

### B — Provenance recovery

Render each representation twice — with and without the evidence facet — and count
how many of six provenance questions (source, method, method version, timestamp,
quality, method limitations) the rendered output can answer. The experiment also
reports *which* questions survive restriction and why. Observed leakage is treated
as a finding: uncertainty notes quote the method, and the assessment facet carries
the observation timestamp, so hiding `E` does not remove provenance — it only
removes the statement of it.

### C — Context dependence

Hold `A` constant (identical responses, identical instrument versions) and change
`C` across three variants: fully declared, thinned, and replaced by a marker. The
mapping policy requires `setting` and `careerStage` before an observation may be
placed in a career direction, so thinner context moves requirements to
`undetermined`, adds `U-CONTEXT-UNDECLARED`, and adds restrictions to the pathway
set. The experiment reports these deltas rather than a narrative.

### D — Temporal stability

Capture the same item set in two sittings three months apart, with the responses
held constant and the situation changed. Compare assessment readings, evidence
timestamps, context deltas, and uncertainty codes for each item pair. A scalar
view reports "no change" for every pair; the representation reports that the
readings did not change while the evidence and context did — which is exactly the
state in which a re-assessment question becomes meaningful.

## What these experiments do not establish

- No claim about ability, suitability, or likely success of any person.
- No test–retest reliability: two sittings are a mechanism check, not a reliability study.
- No generalisation across populations, languages, or instruments.
- The fact enumeration and provenance question sets are authored by this project; a different enumeration would change the percentages while leaving the ordering intact.

## Unresolved methodological questions

1. What is the right instrument-validation procedure for this architecture, and who signs it?
2. How should uncertainty combine across observations without becoming a score by the back door?
3. Which construct set is defensible, given the project vocabulary is not a validated taxonomy?
4. How should a reviewer's comprehension of `(A, E, C) | U` be measured? (Experiment B measures recoverability from output, not human understanding.)
5. What would falsify the 1:3 hypothesis for career assessment specifically?
