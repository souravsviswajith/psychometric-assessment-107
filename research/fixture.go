// Package research is the Research LOOP:
//
//	Observe → Formalize → Transform → Verify → Execute → Compare → Measure → Refine
//
// The experiments are ordinary functions over real pipeline state, so they can
// be executed from a command, from the test suite, and from the app without
// duplicating any logic.
package research

import (
	"fmt"
	"sort"
	"strings"

	"career107/domain"
	"career107/engine"
)

// The deterministic research fixture. Experiments must be repeatable, so the
// fixture uses a fixed clock, a fixed digest algorithm, a fixed subject, and a
// fixed set of instrument items. The fixture is not a persona: it is a set of
// observations with known provenance, used to measure what the architecture
// preserves.

const (
	FixtureStart        = "2025-01-06T09:00:00.000Z"
	FixtureStepMs       = 60_000
	FixtureSubjectID    = "sub_research-fixture"
	FixtureInstrumentID = "instrument.self-description"
)

type ContextEntry struct {
	Key   string
	Value string
}

type FixtureSignalSpec struct {
	ItemRef     string
	Response    domain.RawResponse
	Context     []ContextEntry
	SourceLabel string
}

func option(id string) domain.RawResponse { return domain.RawResponse{Kind: "option", OptionID: id} }

var standardContext = []ContextEntry{
	{"setting", "full-time employment"},
	{"careerStage", "mid-career"},
	{"sessionMode", "self-directed"},
}

var DefaultFixtureSpecs = []FixtureSignalSpec{
	{ItemRef: "itm-selfdesc-analysis-1", Response: option("opt.fully"), Context: standardContext, SourceLabel: "fixture sitting 1"},
	{ItemRef: "itm-skill-quantitative-1", Response: option("opt.mostly"), Context: standardContext, SourceLabel: "fixture sitting 1"},
	{ItemRef: "itm-selfdesc-collaboration-1", Response: option("opt.partly"), Context: standardContext, SourceLabel: "fixture sitting 1"},
	{
		ItemRef:  "itm-context-availability-1",
		Response: option("opt.standard-weekdays"),
		Context: []ContextEntry{
			{"setting", "full-time employment"},
			{"careerStage", "mid-career"},
			{"caringResponsibilities", "school-hours constraint"},
		},
		SourceLabel: "fixture sitting 1",
	},
	{ItemRef: "itm-experience-projects-1", Response: option("opt.often"), Context: standardContext, SourceLabel: "fixture sitting 1"},
}

func FixtureInput(spec FixtureSignalSpec, index int, sessionRef string) domain.SignalInput {
	if sessionRef == "" {
		sessionRef = "session.research-fixture-1"
	}
	declared := make([]domain.DeclaredField, len(spec.Context))
	for i, field := range spec.Context {
		declared[i] = domain.DeclaredField{Key: field.Key, Value: domain.StringValue(field.Value)}
	}
	sequence := index
	return domain.SignalInput{
		InputID:         domain.MintID("inp", map[string]any{"fixture": "research", "itemRef": spec.ItemRef, "index": index, "sessionRef": sessionRef}),
		SubjectID:       FixtureSubjectID,
		InstrumentID:    FixtureInstrumentID,
		ItemRef:         spec.ItemRef,
		Response:        spec.Response,
		DeclaredContext: declared,
		Source: domain.InputSource{
			Kind:  "instrument-administration",
			Ref:   fmt.Sprintf("fixture://%s/%s", spec.SourceLabel, spec.ItemRef),
			Label: spec.SourceLabel,
		},
		SessionRef:     sessionRef,
		ClientSequence: &sequence,
	}
}

type FixtureState struct {
	Runtime         *engine.CareerRuntime
	Signals         []domain.Signal
	Representations []domain.Representation1to3
	PathwaySet      domain.PathwaySet
	FixtureRef      string
}

func FixtureRef() string {
	return fmt.Sprintf("research-fixture@1.0.0(subject=%s)", FixtureSubjectID)
}

type FixtureOptions struct {
	Specs    []FixtureSignalSpec
	StartIso string
	StepMs   int64
}

// BuildFixtureState builds the fixture through the real pipeline: capture (S0)
// → capability → decompose (S1) → capability → map pathways (S2 → S4).
// Experiments therefore measure the system as it runs, not a hand-built
// stand-in.
func BuildFixtureState(options FixtureOptions) (FixtureState, error) {
	specs := options.Specs
	if specs == nil {
		specs = DefaultFixtureSpecs
	}
	start := options.StartIso
	if start == "" {
		start = FixtureStart
	}
	step := options.StepMs
	if step == 0 {
		step = FixtureStepMs
	}
	runtime := engine.New(engine.Options{Clock: domain.SteppingClock(start, step), RuntimeID: "career107-runtime.research"})

	signals := []domain.Signal{}
	for index, spec := range specs {
		captured, rejections := runtime.CaptureSignal(FixtureInput(spec, index, ""), engine.CaptureOptions{})
		if len(rejections) > 0 {
			return FixtureState{}, fmt.Errorf("fixture capture failed for %s: %s", spec.ItemRef, messages(rejections))
		}
		signals = append(signals, captured.Signal)
	}

	representations := []domain.Representation1to3{}
	for _, signal := range signals {
		decomposed, rejections := runtime.DecomposeSignal(signal.ID, engine.InvokeOptions{})
		if len(rejections) > 0 {
			return FixtureState{}, fmt.Errorf("fixture decomposition failed for %s: %s", signal.ID, messages(rejections))
		}
		representations = append(representations, decomposed.Representation)
	}

	ids := make([]string, len(representations))
	for i, representation := range representations {
		ids[i] = representation.ID
	}
	mapped, rejections := runtime.MapPathways(ids, engine.InvokeOptions{})
	if len(rejections) > 0 {
		return FixtureState{}, fmt.Errorf("fixture pathway mapping failed: %s", messages(rejections))
	}
	return FixtureState{
		Runtime:         runtime,
		Signals:         signals,
		Representations: representations,
		PathwaySet:      mapped.PathwaySet,
		FixtureRef:      FixtureRef(),
	}, nil
}

func messages(rejections []domain.Rejection) string {
	parts := make([]string, len(rejections))
	for i, rejection := range rejections {
		parts[i] = rejection.Message
	}
	return strings.Join(parts, "; ")
}

type ReproducibilityReport struct {
	Reproducible bool     `json:"reproducible"`
	Details      []string `json:"details"`
}

// CheckFixtureReproducibility re-runs the same specification twice and reports
// whether the identity held.
func CheckFixtureReproducibility() (ReproducibilityReport, error) {
	first, err := BuildFixtureState(FixtureOptions{})
	if err != nil {
		return ReproducibilityReport{}, err
	}
	second, err := BuildFixtureState(FixtureOptions{})
	if err != nil {
		return ReproducibilityReport{}, err
	}
	details := []string{}

	sortedIDs := func(state FixtureState, representations bool) []string {
		ids := []string{}
		if representations {
			for _, r := range state.Representations {
				ids = append(ids, r.ID)
			}
		} else {
			for _, s := range state.Signals {
				ids = append(ids, s.ID)
			}
		}
		sort.Strings(ids)
		return ids
	}
	equal := func(a, b []string) bool { return strings.Join(a, "\x00") == strings.Join(b, "\x00") }

	firstSignals, secondSignals := sortedIDs(first, false), sortedIDs(second, false)
	signalsMatch := equal(firstSignals, secondSignals)
	if !signalsMatch {
		details = append(details, "signal identities differ between identical runs")
	}
	representationsMatch := equal(sortedIDs(first, true), sortedIDs(second, true))
	if !representationsMatch {
		details = append(details, "representation identities differ between identical runs")
	}
	pathwayIDs := func(state FixtureState) []string {
		ids := []string{}
		for _, p := range state.PathwaySet.Pathways {
			ids = append(ids, p.PathwayID)
		}
		return ids
	}
	pathwaysMatch := equal(pathwayIDs(first), pathwayIDs(second))
	if !pathwaysMatch {
		details = append(details, "pathway candidate sets differ between identical runs")
	}
	if len(details) == 0 {
		details = append(details, fmt.Sprintf("identities reproduced across two runs: %d signals, %d representations", len(firstSignals), len(first.Representations)))
	}
	return ReproducibilityReport{Reproducible: signalsMatch && representationsMatch && pathwaysMatch, Details: details}, nil
}
