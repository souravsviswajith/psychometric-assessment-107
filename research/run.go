package research

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"career107/domain"
)

type ExperimentRun struct {
	Results         []ExperimentResult
	Reproducibility ReproducibilityReport
	Declared        FixtureState
	Thinned         FixtureState
	Removed         FixtureState
	Later           FixtureState
}

// ContextVariantSpecs are the experiment C variants: same responses,
// different context ("declared", "thinned", or "removed").
func ContextVariantSpecs(variant string) []FixtureSignalSpec {
	out := make([]FixtureSignalSpec, len(DefaultFixtureSpecs))
	for i, spec := range DefaultFixtureSpecs {
		next := spec
		switch variant {
		case "thinned":
			next.Context = nil
			for _, field := range spec.Context {
				if field.Key == "setting" {
					next.Context = append(next.Context, field)
				}
			}
		case "removed":
			next.Context = []ContextEntry{{"contextMarker", "not declared"}}
		}
		out[i] = next
	}
	return out
}

func InputsFor(specs []FixtureSignalSpec) []domain.SignalInput {
	inputs := make([]domain.SignalInput, len(specs))
	for i, spec := range specs {
		inputs[i] = FixtureInput(spec, i, "")
	}
	return inputs
}

// LaterSpecs is experiment D's second sitting: identical responses, changed
// situation. The reading should stay the same while context and evidence move.
func LaterSpecs() []FixtureSignalSpec {
	out := make([]FixtureSignalSpec, len(DefaultFixtureSpecs))
	for i, spec := range DefaultFixtureSpecs {
		next := spec
		next.SourceLabel = "fixture sitting 2 (three months later)"
		next.Context = make([]ContextEntry, len(spec.Context))
		for j, field := range spec.Context {
			switch field.Key {
			case "careerStage":
				next.Context[j] = ContextEntry{"careerStage", "mid-career, reassessed after role change"}
			case "setting":
				next.Context[j] = ContextEntry{"setting", "fixed-term contract"}
			default:
				next.Context[j] = field
			}
		}
		out[i] = next
	}
	return out
}

// RunAllExperiments runs every experiment. Fixtures are built through the real
// pipeline, and the reproducibility check runs two identical fixtures to
// confirm deterministic identities.
func RunAllExperiments() (ExperimentRun, error) {
	declared, err := BuildFixtureState(FixtureOptions{Specs: ContextVariantSpecs("declared")})
	if err != nil {
		return ExperimentRun{}, err
	}
	thinned, err := BuildFixtureState(FixtureOptions{Specs: ContextVariantSpecs("thinned"), StartIso: "2025-01-06T09:00:00.000Z"})
	if err != nil {
		return ExperimentRun{}, err
	}
	removed, err := BuildFixtureState(FixtureOptions{Specs: ContextVariantSpecs("removed"), StartIso: "2025-01-06T09:00:00.000Z"})
	if err != nil {
		return ExperimentRun{}, err
	}
	later, err := BuildFixtureState(FixtureOptions{Specs: LaterSpecs(), StartIso: "2025-04-07T09:00:00.000Z"})
	if err != nil {
		return ExperimentRun{}, err
	}
	reproducibility, err := CheckFixtureReproducibility()
	if err != nil {
		return ExperimentRun{}, err
	}
	results := []ExperimentResult{
		RunInformationPreservationExperiment(declared, InputsFor(ContextVariantSpecs("declared"))),
		RunProvenanceRecoveryExperiment(declared),
		RunContextDependenceExperiment(declared, thinned, removed),
		RunTemporalStabilityExperiment(declared, later),
	}
	return ExperimentRun{Results: results, Reproducibility: reproducibility, Declared: declared, Thinned: thinned, Removed: removed, Later: later}, nil
}

// --------------------------------------------------------- machine artifact

type MachineArtifactSignal struct {
	ID          string   `json:"id"`
	ItemRef     string   `json:"itemRef"`
	Uncertainty []string `json:"uncertainty"`
}

// MachineArtifact is the machine-readable result file. The generator and the
// reproducibility checker serialise the same object, so the check always
// tests the artifact that is actually committed.
type MachineArtifact struct {
	// Wall-clock time of the run. Excluded from reproducibility comparison.
	GeneratedAt     string                  `json:"generatedAt"`
	DigestAlgorithm string                  `json:"digestAlgorithm"`
	DigestStrength  string                  `json:"digestStrength"`
	Reproducibility ReproducibilityReport   `json:"reproducibility"`
	Results         []ExperimentResult      `json:"results"`
	PathwayCoverage domain.PathwayCoverage  `json:"pathwayCoverage"`
	FixtureSignals  []MachineArtifactSignal `json:"fixtureSignals"`
}

func BuildMachineArtifact(run ExperimentRun, generatedAt string) MachineArtifact {
	signals := make([]MachineArtifactSignal, len(run.Declared.Signals))
	for i, signal := range run.Declared.Signals {
		codes := make([]string, len(signal.Uncertainty.Notes))
		for j, note := range signal.Uncertainty.Notes {
			codes[j] = note.Code
		}
		signals[i] = MachineArtifactSignal{ID: signal.ID, ItemRef: signal.Instrument.ItemRef, Uncertainty: codes}
	}
	return MachineArtifact{
		GeneratedAt:     generatedAt,
		DigestAlgorithm: domain.SHA256Hasher.Algorithm(),
		DigestStrength:  domain.SHA256Hasher.Strength(),
		Reproducibility: run.Reproducibility,
		Results:         run.Results,
		PathwayCoverage: run.Declared.PathwaySet.Coverage,
		FixtureSignals:  signals,
	}
}

// MarshalArtifact renders the artifact as indented JSON without HTML escaping.
func MarshalArtifact(artifact MachineArtifact) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(artifact); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

// ------------------------------------------------------------------- report
//
// Results rendered as markdown. The report is generated from the results,
// never written by hand, so a claim in the report is always a claim produced
// by a run that actually happened.

func RenderExperimentMarkdown(result ExperimentResult) string {
	measures := make([]string, len(result.Measures))
	for i, m := range result.Measures {
		unit := ""
		if m.Unit != "" {
			unit = " " + m.Unit
		}
		measures[i] = fmt.Sprintf("| %s | %s%s | %s |", m.Label, FormatValue(m.Value), unit, m.Note)
	}
	status := result.Status
	if result.Status == "hypothesis" {
		status += " (architectural hypothesis, not an established psychometric law)"
	}
	bullets := func(items []string) string {
		lines := make([]string, len(items))
		for i, item := range items {
			lines[i] = "- " + item
		}
		return strings.Join(lines, "\n")
	}
	return "### Experiment " + result.ID + " — " + result.Title + "\n\n" +
		"**Status:** " + status + "\n" +
		"**Ran at:** " + result.RanAt + "\n" +
		"**Fixture:** `" + result.FixtureRef + "`\n\n" +
		"**Hypothesis.** " + result.Hypothesis + "\n\n" +
		"**Method.** " + result.Method + "\n\n" +
		"| Measure | Value | Note |\n| --- | --- | --- |\n" +
		strings.Join(measures, "\n") + "\n\n" +
		"**Observations**\n\n" + bullets(result.Observations) + "\n\n" +
		"**Limitations**\n\n" + bullets(result.Limitations) + "\n"
}

func RenderReportMarkdown(results []ExperimentResult, reproducibility *ReproducibilityReport) string {
	repro := ""
	if reproducibility != nil {
		state := "NOT reproducible"
		if reproducibility.Reproducible {
			state = "identical identities across two identical runs"
		}
		repro = "Reproducibility check: " + state + " — " + strings.Join(reproducibility.Details, "; ") + ".\n"
	}
	header := "# Career Assessment 107 — 1:3 verification experiments\n\n" +
		"These experiments test the project's architectural hypothesis that\n" +
		"`raw → (A, E, C) | U` preserves information that `raw → scalar` destroys.\n\n" +
		"They are **not** evidence for a psychometric claim. 1:3 is the project's\n" +
		"architectural hypothesis until empirically validated with instruments that have\n" +
		"their own validation record, and the instrument used in these fixtures is marked\n" +
		"`validated: false`.\n\n" + repro + "\n"
	sections := make([]string, len(results))
	for i, result := range results {
		sections[i] = RenderExperimentMarkdown(result)
	}
	footer := "\n## Methodological position\n\n" +
		"- Results are produced by running the system's real pipeline (capture → capability → decompose → capability → map), not by a simulation of it.\n" +
		"- Every measure is a count or a comparison over recorded representations.\n" +
		"- Where an experiment cannot support a conclusion, the limitation is stated in the experiment itself.\n" +
		"- No experiment in this file supports a claim about a person's ability, suitability, or likely success.\n"
	return header + "\n" + strings.Join(sections, "\n---\n\n") + "\n" + footer
}

// ---------------------------------------------------------------- comparison

// Normalize drops run metadata (generatedAt, ranAt) and keeps every
// measurement, so two runs can be compared exactly.
func Normalize(value any) any {
	switch v := value.(type) {
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = Normalize(item)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for key, item := range v {
			if key == "generatedAt" || key == "ranAt" {
				continue
			}
			out[key] = Normalize(item)
		}
		return out
	case json.Number:
		f, _ := v.Float64()
		return f
	}
	return value
}

// Differences lists up to 10 paths where expected and actual differ.
func Differences(expected, actual any, path string, out []string) []string {
	if len(out) >= 10 {
		return out
	}
	switch e := expected.(type) {
	case []any:
		a, ok := actual.([]any)
		if !ok {
			return append(out, fmt.Sprintf("%s: committed %v, fresh %v", path, expected, actual))
		}
		if len(e) != len(a) {
			out = append(out, fmt.Sprintf("%s.length: committed %d, fresh %d", path, len(e), len(a)))
		}
		for i := 0; i < len(e) && i < len(a); i++ {
			out = Differences(e[i], a[i], fmt.Sprintf("%s[%d]", path, i), out)
		}
		return out
	case map[string]any:
		a, ok := actual.(map[string]any)
		if !ok {
			return append(out, fmt.Sprintf("%s: committed %v, fresh %v", path, expected, actual))
		}
		keys := map[string]bool{}
		for key := range e {
			keys[key] = true
		}
		for key := range a {
			keys[key] = true
		}
		for _, key := range domain.ObjectKeys(toAnyMap(keys)) {
			out = Differences(e[key], a[key], path+"."+key, out)
		}
		return out
	}
	if fmt.Sprint(expected) != fmt.Sprint(actual) {
		out = append(out, fmt.Sprintf("%s: committed %v, fresh %v", path, expected, actual))
	}
	return out
}

func toAnyMap(keys map[string]bool) map[string]any {
	out := map[string]any{}
	for key := range keys {
		out[key] = true
	}
	return out
}

// NormalizeMarkdown drops the "**Ran at:**" lines, which are run metadata.
func NormalizeMarkdown(markdown string) string {
	lines := []string{}
	for _, line := range strings.Split(markdown, "\n") {
		if !strings.HasPrefix(line, "**Ran at:**") {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
