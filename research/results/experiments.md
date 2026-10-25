# Career Assessment 107 — 1:3 verification experiments

These experiments test the project's architectural hypothesis that
`raw → (A, E, C) | U` preserves information that `raw → scalar` destroys.

They are **not** evidence for a psychometric claim. 1:3 is the project's
architectural hypothesis until empirically validated with instruments that have
their own validation record, and the instrument used in these fixtures is marked
`validated: false`.

Reproducibility check: identical identities across two identical runs — identities reproduced across two runs: 5 signals, 5 representations.


### Experiment A — Information preservation: raw → scalar vs raw → (A, E, C) | U

**Status:** measured
**Ran at:** 2026-10-03T09:50:38.605Z
**Fixture:** `research-fixture@1.0.0(subject=sub_research-fixture)`

**Hypothesis.** Compressing an observation into a single scalar discards provenance, method, temporal, quality, context, and uncertainty information that a reviewer needs in order to interpret it.

**Method.** Enumerate recoverable facts from each raw observation, then check which of those facts are recoverable from the 1:3 representation and from a scalar mean of response positions. Compare counts.

| Measure | Value | Note |
| --- | --- | --- |
| facts enumerated from the raw observations | 209 facts |  |
| facts recoverable from (A, E, C) | U | 204 facts | recoverability is checked by set membership against the primitive values recorded in the representation |
| facts recoverable from the scalar compression | 0 facts | the scalar is 0.7500 (mean of 5 positions) plus an observation count |
| information lost by scalar compression | 100 % |  |
| scalar value | 0.75 | shown to make the compression concrete; it is not stored anywhere in the system |

**Observations**

- Each observation contributes 42 recoverable facts on average.
- Not every enumerated fact is recoverable, and the exceptions are recorded rather than hidden: source.label. The human-readable source label is a display convenience; the representation keeps the stable source reference instead.
- Scalar compression retains the aggregate reading and the count, but none of the enumerated facts: provenance, method version, timestamps, quality metadata, context fields, and uncertainty are all unrecoverable from it.
- The (A, E, C) | U representation retains the raw response alongside its interpretation policy, so a reviewer can recompute the reading rather than trust it.

**Limitations**

- Fact counts are a proxy for information content: they measure the recoverability of enumerated facts, not the semantic value of the information.
- The enumeration is authored by this project; a different enumeration would change the percentages while leaving the ordering (1:3 ≫ scalar) unchanged.
- This experiment says nothing about whether the facts are the *right* facts to record for career assessment.

---

### Experiment B — Provenance recovery with and without the E facet

**Status:** measured
**Ran at:** 2026-10-03T09:50:38.607Z
**Fixture:** `research-fixture@1.0.0(subject=sub_research-fixture)`

**Hypothesis.** Removing the evidence facet destroys a reviewer’s ability to establish source, method, timestamp, version, and evidence quality.

**Method.** Render every representation twice (E present, E restricted) and count how many of six provenance questions the rendered output can answer.

| Measure | Value | Note |
| --- | --- | --- |
| provenance questions per representation | 6 questions |  |
| questions answerable with E present | 30 answers | of 30 |
| questions answerable with E restricted | 10 answers | of 30 |
| provenance recovery rate with E present | 100 % |  |
| provenance recovery rate with E restricted | 33.3 % |  |
| weakest provenance question | Where did this observation come from? | the question least often answerable from a full render |

**Observations**

- With E present, every provenance question is answerable, including the method’s declared limitations.
- With E restricted, the questions that remain answerable are: method, timestamp. Where a question survives restriction it is because another facet leaked the answer, not because E is unnecessary.
- The observed leakage is informative: the assessment facet carries the observation timestamp, and uncertainty notes quote the collection method in their detail text. Provenance therefore cannot be "removed" by hiding one facet — it has to be replaced by an explicit statement of what is unknown.
- The residual (what survives restriction) is exactly the part of a claim that can be copied out of the system and still look authoritative — which is why provenance is a first-class facet rather than a UI detail.

**Limitations**

- The questions are authored by this project; a different question set would produce different rates.
- The renderer is the system’s own reader: a reviewer reading raw JSON might recover more or less.
- This measures recoverability from the representation, not whether a human reviewer actually reads the evidence.

---

### Experiment C — Context dependence with the assessment held constant

**Status:** measured
**Ran at:** 2026-10-03T09:50:38.607Z
**Fixture:** `research-fixture@1.0.0(subject=sub_research-fixture) + context variants`

**Hypothesis.** Interpretation that is legitimately context-dependent must change when C changes while A is held constant, and the change must be explicit rather than silent.

**Method.** Build three fixtures with identical responses and progressively thinner context, map each to pathways through the real capability, and compare requirement statuses, uncertainty codes, and pathway restrictions.

| Measure | Value | Note |
| --- | --- | --- |
| assessment facet identical across context variants | true | A held constant by construction |
| context facets differ | true |  |
| requirement statuses that changed to "undetermined" when context was removed | 8 requirements |  |
| new uncertainty codes when context was removed | U-CONTEXT-UNDECLARED |  |
| restrictions attached to pathways (declared → removed) | 2 → 3 restriction entries |  |

**Observations**

- context declared: supported 13, gap 0, conflicting 0, undetermined 27
- context thinned: supported 5, gap 0, conflicting 0, undetermined 35
- context removed: supported 5, gap 0, conflicting 0, undetermined 35
- Holding A constant while changing C changes what the mapping is willing to say: requirements move to "undetermined" and additional uncertainty is recorded.
- The mapping never silently absorbs the context change: it reports the change through uncertainty notes, requirement rationales, and pathway restrictions.

**Limitations**

- The variants are synthetic context edits, not observed changes in a person’s situation.
- Only the pathway mapping is measured; other transformations (for example model-assisted explanation) are not exercised here.
- A finding that context matters for these items does not generalise to all career constructs.

---

### Experiment D — Temporal comparison with assessment, evidence, and context tracked separately

**Status:** measured
**Ran at:** 2026-10-03T09:50:38.608Z
**Fixture:** `research-fixture@1.0.0(subject=sub_research-fixture) vs later sitting`

**Hypothesis.** An unchanged reading across two sittings is not the same as an unchanged observation: evidence and context move independently, and the representation must show that separately.

**Method.** Capture and decompose the same item set twice with a fixed clock step, then compare the assessment reading, the evidence timestamps, the context delta, and the uncertainty codes for each item pair.

| Measure | Value | Note |
| --- | --- | --- |
| observations compared | 5 pairs |  |
| pairs with a changed assessment reading | 0 pairs |  |
| pairs where the scalar view would report "no change" | 5 pairs | a scalar comparison sees only the reading, not the evidence or context that moved |
| pairs with a later evidence timestamp | 5 pairs |  |
| pairs with changed context | 5 pairs |  |
| pairs with changed uncertainty profile | 0 pairs |  |
| instrument version held constant | true |  |

**Observations**

- Assessment readings changed in 0 of 5 pairs, while the evidence timestamps moved in 5 and the context changed in 5.
- The three dimensions are tracked separately, so "the answer did not change but the situation did" is a representable state rather than an invisible one.
- A re-assessment decision can therefore cite the temporal and contextual evidence rather than the reading alone — and the decision itself remains the user’s.

**Limitations**

- The fixture’s second sitting reuses the same responses; this measures representation behaviour, not test–retest reliability of any instrument.
- Two sittings cannot establish temporal stability; the experiment is a mechanism check, not a reliability study.
- The instrument is unvalidated (see uncertainty notes), so no psychometric claim is made from these numbers.


## Methodological position

- Results are produced by running the system's real pipeline (capture → capability → decompose → capability → map), not by a simulation of it.
- Every measure is a count or a comparison over recorded representations.
- Where an experiment cannot support a conclusion, the limitation is stated in the experiment itself.
- No experiment in this file supports a claim about a person's ability, suitability, or likely success.
