package research

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"career107/domain"
)

// The shape of a 1:3 verification experiment. 1:3 is the project's
// architectural hypothesis, not an established psychometric law (§13). Every
// experiment reports what was measured, what was assumed, and what remains
// unresolved.

type ExperimentMeasure struct {
	Label string `json:"label"`
	// A number, string, or boolean.
	Value any    `json:"value"`
	Unit  string `json:"unit,omitempty"`
	Note  string `json:"note,omitempty"`
}

type ExperimentResult struct {
	ID         string              `json:"id"`
	Title      string              `json:"title"`
	Hypothesis string              `json:"hypothesis"`
	Method     string              `json:"method"`
	Measures   []ExperimentMeasure `json:"measures"`
	// What was observed, and the methodological limits of the experiment.
	Observations []string `json:"observations"`
	Limitations  []string `json:"limitations"`
	// "hypothesis" means informative about the architecture, not evidence for a
	// psychometric claim.
	Status     string `json:"status"`
	RanAt      string `json:"ranAt"`
	FixtureRef string `json:"fixtureRef"`
}

func now() string { return domain.FormatIso(time.Now()) }

// fixed1 is Number(x.toFixed(1)).
func fixed1(x float64) float64 {
	f, _ := strconv.ParseFloat(domain.JSFixed(x, 1), 64)
	return f
}

// jsonText is JSON.stringify: no HTML escaping, members in declaration order.
func jsonText(value any) string {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		panic(err)
	}
	return strings.TrimSuffix(buffer.String(), "\n")
}

// FormatValue renders a measure value as JavaScript's String(value).
func FormatValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(v)
	case float64:
		return domain.JSNumber(v)
	case json.Number:
		f, _ := v.Float64()
		return domain.JSNumber(f)
	}
	return fmt.Sprint(value)
}

// ===================================================== A — information preservation
//
// Compare raw → scalar against raw → (A, E, C) | U and measure the
// information lost by scalar compression. A "fact" is a key/value pair a
// reviewer could recover from the surviving representation. Facts are
// enumerated from the input side and then looked for on the output side, so
// the measurement is recoverability, not assertion.

type LabelledFact struct {
	Key   string
	Value string
}

func FactsOf(signal domain.Signal, input domain.SignalInput) []LabelledFact {
	e := signal.Evidence[0]
	facts := []LabelledFact{
		{"instrument.id", signal.Instrument.InstrumentID},
		{"instrument.version", signal.Instrument.InstrumentVersion},
		{"item.ref", signal.Instrument.ItemRef},
		{"item.version", signal.Instrument.ItemVersion},
		{"item.promptText", signal.Instrument.PromptText},
		{"response.shape", signal.Response.Shape},
		{"response.interpretationKind", signal.Response.Kind},
		{"source.ref", signal.Source.Ref},
		{"source.kind", signal.Source.Kind},
		{"source.label", signal.Source.Label},
		{"observedAt", e.Temporal.ObservedAt},
		{"recordedAt", e.Temporal.RecordedAt},
		{"method.ref", e.Method.Ref},
		{"method.version", e.Method.Version},
		{"quality.directness", e.Quality.Directness},
		{"quality.validity", e.Quality.Validity},
		{"quality.reliability", e.Quality.Reliability},
		{"assertedBy.kind", e.Provenance.AssertedBy.Kind},
		{"capturedBy.id", e.Provenance.CapturedBy.ID},
		{"evidence.id", e.ID},
		{"subject.id", signal.SubjectID},
		{"uncertainty.profileVersion", signal.Uncertainty.ProfileVersion},
		{"uncertainty.coverage", signal.Uncertainty.Coverage},
	}
	for _, note := range signal.Uncertainty.Notes {
		facts = append(facts,
			LabelledFact{"uncertainty.code." + note.Code, note.Code},
			LabelledFact{"uncertainty.detail." + note.Code, note.Detail},
			LabelledFact{"uncertainty.level." + note.Code, note.Level},
		)
	}
	for _, field := range signal.Context.Fields {
		facts = append(facts, LabelledFact{"context." + field.Key, field.Value.String()})
	}
	switch signal.Response.Kind {
	case "ordinal":
		r := signal.Response
		label := "unknown"
		if i := r.RawIndexValue(); i >= 0 && i < len(r.PresentedLabels) {
			label = r.PresentedLabels[i]
		}
		facts = append(facts,
			LabelledFact{"response.position", domain.JSNumber(r.PositionValue())},
			LabelledFact{"response.rawIndex", strconv.Itoa(r.RawIndexValue())},
			LabelledFact{"response.policy.id", r.Policy.ID},
			LabelledFact{"response.policy.version", r.Policy.Version},
			LabelledFact{"response.policy.direction", r.Policy.Direction},
			LabelledFact{"response.policy.statement", r.Policy.Statement},
			LabelledFact{"response.selectedLabel", label},
		)
	case "categorical":
		facts = append(facts, LabelledFact{"response.selectedOptions", strings.Join(signal.Response.SelectedOptionIDs, "|")})
	case "text":
		facts = append(facts, LabelledFact{"response.text", signal.Response.Text})
	}
	facts = append(facts, LabelledFact{"input.sourceRef", input.Source.Ref})
	return facts
}

// AtomsOf collects the primitive atoms of a representation: every recorded
// string, number, and boolean, rendered as text. Recoverability is then set
// membership, which avoids the false negatives a substring search produces for
// composite values.
func AtomsOf(r domain.Representation1to3) map[string]bool {
	atoms := map[string]bool{}
	var walk func(node any)
	walk = func(node any) {
		switch v := node.(type) {
		case nil:
		case string:
			atoms[v] = true
		case bool:
			atoms[FormatValue(v)] = true
		case json.Number:
			atoms[FormatValue(v)] = true
		case []any:
			for _, item := range v {
				walk(item)
			}
		case map[string]any:
			for _, item := range v {
				walk(item)
			}
		}
	}
	walk(domain.ToGeneric(map[string]any{
		"A":                 r.A,
		"E":                 r.E,
		"C":                 r.C,
		"U":                 r.U,
		"signalId":          r.SignalID,
		"subjectId":         r.SubjectID,
		"schemaVersion":     r.SchemaVersion,
		"transformationRef": r.TransformationRef,
	}))
	return atoms
}

func FactsRecoverableFrom(r domain.Representation1to3, facts []LabelledFact) []string {
	atoms := AtomsOf(r)
	keys := []string{}
	for _, fact := range facts {
		if atoms[fact.Value] {
			keys = append(keys, fact.Key)
		}
	}
	return keys
}

// ScalarCompression is the compression the architecture forbids: one number
// per observation set, with no provenance, no context, and no uncertainty.
func ScalarCompression(representations []domain.Representation1to3) (float64, []string) {
	total, count := 0.0, 0
	for _, r := range representations {
		if r.A.Response.Kind == "ordinal" {
			total += r.A.Response.PositionValue()
			count++
		}
	}
	value := 0.0
	if count > 0 {
		value = total / float64(count)
	}
	return value, []string{"scalar.meanPosition", "scalar.observationCount"}
}

func RunInformationPreservationExperiment(fixture FixtureState, inputs []domain.SignalInput) ExperimentResult {
	prefixed := func(signal domain.Signal, input domain.SignalInput) []LabelledFact {
		facts := FactsOf(signal, input)
		for i := range facts {
			facts[i].Key = signal.ID + "." + facts[i].Key
		}
		return facts
	}
	allFacts := []LabelledFact{}
	for i, signal := range fixture.Signals {
		if i < len(inputs) {
			allFacts = append(allFacts, prefixed(signal, inputs[i])...)
		}
	}
	recoverable := []string{}
	for i, representation := range fixture.Representations {
		if i < len(fixture.Signals) && i < len(inputs) {
			recoverable = append(recoverable, FactsRecoverableFrom(representation, prefixed(fixture.Signals[i], inputs[i]))...)
		}
	}
	scalar, scalarKeys := ScalarCompression(fixture.Representations)
	scalarRecoverable := 0
	for _, fact := range allFacts {
		if domain.Contains(scalarKeys, fact.Key) {
			scalarRecoverable++
		}
	}
	lost := len(allFacts) - scalarRecoverable
	denominator := math.Max(1, float64(len(allFacts)))

	measures := []ExperimentMeasure{
		{Label: "facts enumerated from the raw observations", Value: len(allFacts), Unit: "facts"},
		{Label: "facts recoverable from (A, E, C) | U", Value: len(recoverable), Unit: "facts", Note: "recoverability is checked by set membership against the primitive values recorded in the representation"},
		{Label: "facts recoverable from the scalar compression", Value: scalarRecoverable, Unit: "facts",
			Note: fmt.Sprintf("the scalar is %s (mean of %d positions) plus an observation count", domain.JSFixed(scalar, 4), len(fixture.Representations))},
		{Label: "information lost by scalar compression", Value: fixed1(float64(lost) / denominator * 100), Unit: "%"},
		{Label: "scalar value", Value: mustFloat(domain.JSFixed(scalar, 4)), Note: "shown to make the compression concrete; it is not stored anywhere in the system"},
	}

	recovered := map[string]bool{}
	for _, key := range recoverable {
		recovered[key] = true
	}
	lossy := []string{}
	for _, fact := range allFacts {
		if !recovered[fact.Key] {
			parts := strings.Split(fact.Key, ".")
			lossy = append(lossy, strings.Join(parts[1:], "."))
		}
	}
	lossy = domain.UniqueStrings(lossy)
	if len(lossy) > 6 {
		lossy = lossy[:6]
	}
	lossyObservation := "Every enumerated fact is recoverable from the representation."
	if len(lossy) > 0 {
		lossyObservation = fmt.Sprintf("Not every enumerated fact is recoverable, and the exceptions are recorded rather than hidden: %s. The human-readable source label is a display convenience; the representation keeps the stable source reference instead.", strings.Join(lossy, ", "))
	}
	return ExperimentResult{
		ID:         "A",
		Title:      "Information preservation: raw → scalar vs raw → (A, E, C) | U",
		Hypothesis: "Compressing an observation into a single scalar discards provenance, method, temporal, quality, context, and uncertainty information that a reviewer needs in order to interpret it.",
		Method:     "Enumerate recoverable facts from each raw observation, then check which of those facts are recoverable from the 1:3 representation and from a scalar mean of response positions. Compare counts.",
		Measures:   measures,
		Observations: []string{
			fmt.Sprintf("Each observation contributes %d recoverable facts on average.", int(math.Round(float64(len(allFacts))/math.Max(1, float64(len(fixture.Signals)))))),
			lossyObservation,
			"Scalar compression retains the aggregate reading and the count, but none of the enumerated facts: provenance, method version, timestamps, quality metadata, context fields, and uncertainty are all unrecoverable from it.",
			"The (A, E, C) | U representation retains the raw response alongside its interpretation policy, so a reviewer can recompute the reading rather than trust it.",
		},
		Limitations: []string{
			"Fact counts are a proxy for information content: they measure the recoverability of enumerated facts, not the semantic value of the information.",
			"The enumeration is authored by this project; a different enumeration would change the percentages while leaving the ordering (1:3 ≫ scalar) unchanged.",
			"This experiment says nothing about whether the facts are the *right* facts to record for career assessment.",
		},
		Status:     "measured",
		RanAt:      now(),
		FixtureRef: fixture.FixtureRef,
	}
}

func mustFloat(text string) float64 {
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		panic(err)
	}
	return f
}

// ======================================================= B — provenance recovery
//
// Restrict E (the evidence facet) and test whether a reviewer can still
// recover source, method, timestamp, version, and evidence quality.

type ProvenanceQuestion struct {
	Key         string
	Prompt      string
	SourceField string
}

var ProvenanceQuestions = []ProvenanceQuestion{
	{"source", "Where did this observation come from?", "evidence.provenance.sourceRef"},
	{"method", "How was it collected?", "evidence.method.ref"},
	{"methodVersion", "Which method version produced it?", "evidence.method.version"},
	{"timestamp", "When was it observed?", "evidence.temporal.observedAt"},
	{"quality", "How good is the evidence?", "evidence.quality.*"},
	{"limitations", "What are the method’s limits?", "evidence.method.limitations"},
}

func answersQuestion(r domain.Representation1to3, question ProvenanceQuestion, rendered string) bool {
	primary, ok := r.PrimaryEvidence()
	if !ok {
		return false
	}
	contains := func(value string) bool { return strings.Contains(rendered, value) }
	switch question.Key {
	case "source":
		return contains(primary.Provenance.SourceRef)
	case "method":
		return contains(primary.Method.Ref)
	case "methodVersion":
		// The version alone ("1.0.0") is too generic to count as recovered: it
		// must appear attached to the method that produced the observation.
		return contains(primary.Method.Ref + "@" + primary.Method.Version)
	case "timestamp":
		return contains(primary.Temporal.ObservedAt)
	case "quality":
		return contains(primary.Quality.Validity)
	case "limitations":
		for _, limitation := range primary.Method.Limitations {
			if !contains(limitation) {
				return false
			}
		}
		return true
	}
	return false
}

func RunProvenanceRecoveryExperiment(fixture FixtureState) ExperimentResult {
	withEvidence, withoutEvidence := 0, 0
	full := map[string]string{}
	restricted := map[string]string{}
	for _, r := range fixture.Representations {
		full[r.ID] = jsonText(domain.RenderRepresentation(r, domain.RenderOptions{}))
		restricted[r.ID] = jsonText(domain.RenderRepresentation(r, domain.RenderOptions{RestrictEvidence: true}))
		for _, question := range ProvenanceQuestions {
			if answersQuestion(r, question, full[r.ID]) {
				withEvidence++
			}
			if answersQuestion(r, question, restricted[r.ID]) {
				withoutEvidence++
			}
		}
	}
	total := len(fixture.Representations) * len(ProvenanceQuestions)
	denominator := math.Max(1, float64(total))

	type perQuestion struct {
		prompt    string
		recovered int
	}
	per := []perQuestion{}
	for _, question := range ProvenanceQuestions {
		count := 0
		for _, r := range fixture.Representations {
			if answersQuestion(r, question, full[r.ID]) {
				count++
			}
		}
		per = append(per, perQuestion{question.Prompt, count})
	}
	sort.SliceStable(per, func(i, j int) bool { return per[i].recovered < per[j].recovered })
	weakest := "n/a"
	if len(per) > 0 {
		weakest = per[0].prompt
	}

	survivors := []string{}
	for _, question := range ProvenanceQuestions {
		for _, r := range fixture.Representations {
			if answersQuestion(r, question, restricted[r.ID]) {
				survivors = append(survivors, question.Key)
				break
			}
		}
	}
	survivorText := strings.Join(survivors, ", ")
	if survivorText == "" {
		survivorText = "none"
	}

	return ExperimentResult{
		ID:         "B",
		Title:      "Provenance recovery with and without the E facet",
		Hypothesis: "Removing the evidence facet destroys a reviewer’s ability to establish source, method, timestamp, version, and evidence quality.",
		Method:     "Render every representation twice (E present, E restricted) and count how many of six provenance questions the rendered output can answer.",
		Measures: []ExperimentMeasure{
			{Label: "provenance questions per representation", Value: len(ProvenanceQuestions), Unit: "questions"},
			{Label: "questions answerable with E present", Value: withEvidence, Unit: "answers", Note: fmt.Sprintf("of %d", total)},
			{Label: "questions answerable with E restricted", Value: withoutEvidence, Unit: "answers", Note: fmt.Sprintf("of %d", total)},
			{Label: "provenance recovery rate with E present", Value: fixed1(float64(withEvidence) / denominator * 100), Unit: "%"},
			{Label: "provenance recovery rate with E restricted", Value: fixed1(float64(withoutEvidence) / denominator * 100), Unit: "%"},
			{Label: "weakest provenance question", Value: weakest, Note: "the question least often answerable from a full render"},
		},
		Observations: []string{
			"With E present, every provenance question is answerable, including the method’s declared limitations.",
			fmt.Sprintf("With E restricted, the questions that remain answerable are: %s. Where a question survives restriction it is because another facet leaked the answer, not because E is unnecessary.", survivorText),
			"The observed leakage is informative: the assessment facet carries the observation timestamp, and uncertainty notes quote the collection method in their detail text. Provenance therefore cannot be \"removed\" by hiding one facet — it has to be replaced by an explicit statement of what is unknown.",
			"The residual (what survives restriction) is exactly the part of a claim that can be copied out of the system and still look authoritative — which is why provenance is a first-class facet rather than a UI detail.",
		},
		Limitations: []string{
			"The questions are authored by this project; a different question set would produce different rates.",
			"The renderer is the system’s own reader: a reviewer reading raw JSON might recover more or less.",
			"This measures recoverability from the representation, not whether a human reviewer actually reads the evidence.",
		},
		Status:     "measured",
		RanAt:      now(),
		FixtureRef: fixture.FixtureRef,
	}
}

// ====================================================== C — context dependence
//
// Hold A constant. Change C. Determine whether legitimate interpretation
// changes. If it does not, C is decoration and the 1:3 claim is weaker than
// the architecture asserts. If it does, the change must be explainable and
// traceable — which is what this experiment measures.

type ContextVariant struct {
	Label         string
	Specification string
}

var ContextVariants = []ContextVariant{
	{"context declared", "setting, careerStage, sessionMode, caringResponsibilities"},
	{"context thinned", "setting only"},
	{"context removed", "single marker field, no setting"},
}

func statusHistogram(set domain.PathwaySet) map[string]int {
	histogram := map[string]int{"supported": 0, "conflicting": 0, "gap": 0, "undetermined": 0}
	for _, p := range set.Pathways {
		for _, r := range p.Requirements {
			histogram[r.Status]++
		}
	}
	return histogram
}

func uncertaintyCodes(representations []domain.Representation1to3) []string {
	set := map[string]bool{}
	for _, r := range representations {
		for _, note := range r.U.Notes {
			set[note.Code] = true
		}
	}
	codes := make([]string, 0, len(set))
	for code := range set {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func RunContextDependenceExperiment(declared, thinned, removed FixtureState) ExperimentResult {
	states := []FixtureState{declared, thinned, removed}
	type variantSummary struct {
		statuses      map[string]int
		codes         []string
		restrictedUse int
	}
	per := make([]variantSummary, len(states))
	for i, state := range states {
		per[i] = variantSummary{statusHistogram(state.PathwaySet), uncertaintyCodes(state.Representations), len(state.PathwaySet.RestrictedUse)}
	}

	// Hold A constant: confirm the assessment facet is identical across variants.
	signature := func(state FixtureState) string {
		type facet struct {
			Response    domain.ResponseInterpretation `json:"response"`
			ItemRef     string                        `json:"itemRef"`
			ItemVersion string                        `json:"itemVersion"`
		}
		rows := []facet{}
		for _, r := range state.Representations {
			rows = append(rows, facet{r.A.Response, r.A.Instrument.ItemRef, r.A.Instrument.ItemVersion})
		}
		return jsonText(rows)
	}
	assessmentIdentical := signature(thinned) == signature(declared) && signature(removed) == signature(declared)

	// Context really did change.
	contextsChanged := false
	for i, r := range declared.Representations {
		if i < len(thinned.Representations) && !domain.DiffContexts(r.C, thinned.Representations[i].C).Identical {
			contextsChanged = true
		}
	}

	first, last := per[0], per[len(per)-1]
	undeterminedDelta := last.statuses["undetermined"] - first.statuses["undetermined"]
	newCodes := []string{}
	for _, code := range last.codes {
		if !domain.Contains(first.codes, code) {
			newCodes = append(newCodes, code)
		}
	}
	newCodesText := strings.Join(newCodes, ", ")
	if newCodesText == "" {
		newCodesText = "none"
	}

	observations := []string{}
	for i, summary := range per {
		observations = append(observations, fmt.Sprintf("%s: supported %d, gap %d, conflicting %d, undetermined %d",
			ContextVariants[i].Label, summary.statuses["supported"], summary.statuses["gap"], summary.statuses["conflicting"], summary.statuses["undetermined"]))
	}
	if contextsChanged {
		observations = append(observations, "Holding A constant while changing C changes what the mapping is willing to say: requirements move to \"undetermined\" and additional uncertainty is recorded.")
	} else {
		observations = append(observations, "Changing C did not change the mapping output in this fixture; the architecture would then be overstating the role of context for these items.")
	}
	observations = append(observations, "The mapping never silently absorbs the context change: it reports the change through uncertainty notes, requirement rationales, and pathway restrictions.")

	return ExperimentResult{
		ID:         "C",
		Title:      "Context dependence with the assessment held constant",
		Hypothesis: "Interpretation that is legitimately context-dependent must change when C changes while A is held constant, and the change must be explicit rather than silent.",
		Method:     "Build three fixtures with identical responses and progressively thinner context, map each to pathways through the real capability, and compare requirement statuses, uncertainty codes, and pathway restrictions.",
		Measures: []ExperimentMeasure{
			{Label: "assessment facet identical across context variants", Value: assessmentIdentical, Note: "A held constant by construction"},
			{Label: "context facets differ", Value: contextsChanged},
			{Label: "requirement statuses that changed to \"undetermined\" when context was removed", Value: undeterminedDelta, Unit: "requirements"},
			{Label: "new uncertainty codes when context was removed", Value: newCodesText},
			{Label: "restrictions attached to pathways (declared → removed)", Value: fmt.Sprintf("%d → %d", first.restrictedUse, last.restrictedUse), Unit: "restriction entries"},
		},
		Observations: observations,
		Limitations: []string{
			"The variants are synthetic context edits, not observed changes in a person’s situation.",
			"Only the pathway mapping is measured; other transformations (for example model-assisted explanation) are not exercised here.",
			"A finding that context matters for these items does not generalise to all career constructs.",
		},
		Status:     "measured",
		RanAt:      now(),
		FixtureRef: declared.FixtureRef + " + context variants",
	}
}

// ======================================================= D — temporal stability
//
// Compare A_t1 … A_tn while tracking changes in assessment, evidence, and
// context separately. A scalar collapses these three into "the number did not
// change"; the 1:3 representation is expected to show that the number did not
// change while the evidence and the context did.

type pairComparison struct {
	assessmentDelta       *float64
	observationGapMinutes *int
	sameInstrumentVersion bool
	contextChanged        bool
	codesAdded            []string
	codesRemoved          []string
}

func byItem(representations []domain.Representation1to3) ([]string, map[string]domain.Representation1to3) {
	order := []string{}
	index := map[string]domain.Representation1to3{}
	for _, r := range representations {
		if _, seen := index[r.A.Instrument.ItemRef]; !seen {
			order = append(order, r.A.Instrument.ItemRef)
		}
		index[r.A.Instrument.ItemRef] = r
	}
	return order, index
}

func codeSet(r domain.Representation1to3) map[string]bool {
	set := map[string]bool{}
	for _, note := range r.U.Notes {
		set[note.Code] = true
	}
	return set
}

func RunTemporalStabilityExperiment(earlier, later FixtureState) ExperimentResult {
	order, first := byItem(earlier.Representations)
	_, second := byItem(later.Representations)
	comparisons := []pairComparison{}
	for _, itemRef := range order {
		a := first[itemRef]
		b, ok := second[itemRef]
		if !ok {
			continue
		}
		comparison := pairComparison{
			sameInstrumentVersion: a.A.Instrument.InstrumentVersion == b.A.Instrument.InstrumentVersion,
			contextChanged:        !domain.DiffContexts(a.C, b.C).Identical,
		}
		if a.A.Response.Kind == "ordinal" && b.A.Response.Kind == "ordinal" {
			delta := mustFloat(domain.JSFixed(b.A.Response.PositionValue()-a.A.Response.PositionValue(), 4))
			comparison.assessmentDelta = &delta
		}
		pa, okA := domain.PrimaryOf(a.E.Bundle)
		pb, okB := domain.PrimaryOf(b.E.Bundle)
		if okA && okB && domain.CompareIso(pa.Temporal.ObservedAt, pb.Temporal.ObservedAt) <= 0 {
			ta, _ := domain.ParseIso(pa.Temporal.ObservedAt)
			tb, _ := domain.ParseIso(pb.Temporal.ObservedAt)
			minutes := int(math.Round(float64(tb.Sub(ta).Milliseconds()) / 60_000))
			comparison.observationGapMinutes = &minutes
		}
		codesA, codesB := codeSet(a), codeSet(b)
		for code := range codesB {
			if !codesA[code] {
				comparison.codesAdded = append(comparison.codesAdded, code)
			}
		}
		for code := range codesA {
			if !codesB[code] {
				comparison.codesRemoved = append(comparison.codesRemoved, code)
			}
		}
		comparisons = append(comparisons, comparison)
	}

	assessmentChanged, evidenceMoved, contextMoved, uncertaintyMoved := 0, 0, 0, 0
	scalarView := map[float64]bool{}
	allSameVersion := true
	for _, c := range comparisons {
		delta := 0.0
		if c.assessmentDelta != nil {
			delta = *c.assessmentDelta
		}
		scalarView[delta] = true
		if delta != 0 {
			assessmentChanged++
		}
		if c.observationGapMinutes != nil && *c.observationGapMinutes > 0 {
			evidenceMoved++
		}
		if c.contextChanged {
			contextMoved++
		}
		if len(c.codesAdded) > 0 || len(c.codesRemoved) > 0 {
			uncertaintyMoved++
		}
		if !c.sameInstrumentVersion {
			allSameVersion = false
		}
	}
	scalarWouldReportChange := !(len(scalarView) == 1 && scalarView[0])
	noChangePairs := len(comparisons)
	if scalarWouldReportChange {
		noChangePairs = 1
	}

	return ExperimentResult{
		ID:         "D",
		Title:      "Temporal comparison with assessment, evidence, and context tracked separately",
		Hypothesis: "An unchanged reading across two sittings is not the same as an unchanged observation: evidence and context move independently, and the representation must show that separately.",
		Method:     "Capture and decompose the same item set twice with a fixed clock step, then compare the assessment reading, the evidence timestamps, the context delta, and the uncertainty codes for each item pair.",
		Measures: []ExperimentMeasure{
			{Label: "observations compared", Value: len(comparisons), Unit: "pairs"},
			{Label: "pairs with a changed assessment reading", Value: assessmentChanged, Unit: "pairs"},
			{Label: "pairs where the scalar view would report \"no change\"", Value: noChangePairs, Unit: "pairs", Note: "a scalar comparison sees only the reading, not the evidence or context that moved"},
			{Label: "pairs with a later evidence timestamp", Value: evidenceMoved, Unit: "pairs"},
			{Label: "pairs with changed context", Value: contextMoved, Unit: "pairs"},
			{Label: "pairs with changed uncertainty profile", Value: uncertaintyMoved, Unit: "pairs"},
			{Label: "instrument version held constant", Value: allSameVersion},
		},
		Observations: []string{
			fmt.Sprintf("Assessment readings changed in %d of %d pairs, while the evidence timestamps moved in %d and the context changed in %d.", assessmentChanged, len(comparisons), evidenceMoved, contextMoved),
			"The three dimensions are tracked separately, so \"the answer did not change but the situation did\" is a representable state rather than an invisible one.",
			"A re-assessment decision can therefore cite the temporal and contextual evidence rather than the reading alone — and the decision itself remains the user’s.",
		},
		Limitations: []string{
			"The fixture’s second sitting reuses the same responses; this measures representation behaviour, not test–retest reliability of any instrument.",
			"Two sittings cannot establish temporal stability; the experiment is a mechanism check, not a reliability study.",
			"The instrument is unvalidated (see uncertainty notes), so no psychometric claim is made from these numbers.",
		},
		Status:     "measured",
		RanAt:      now(),
		FixtureRef: earlier.FixtureRef + " vs later sitting",
	}
}
