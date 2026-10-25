package domain

import (
	"fmt"
	"math"
	"strings"
)

// The uncertainty vocabulary.
//
// U is cross-cutting metadata, not a fourth semantic dimension (§3):
//
//	x_i → (A_i, E_i, C_i) | U_i
//
// Every uncertainty note must name a registered code. Free-form uncertainty is
// not accepted, because an unregistered note cannot be compared across time,
// transformed, or aggregated by the research experiments.

var UncertaintySources = []string{
	"response-ambiguity",
	"self-report-bias",
	"instrument-coverage",
	"item-wording",
	"memory-recall",
	"context-ambiguity",
	"aggregation",
	"transformation-loss",
	"model-generation",
	"stale-observation",
	"translation",
	"method-limitation",
}

var UncertaintyKinds = []string{
	"epistemic", // reducible in principle: more or better evidence would reduce it
	"aleatoric", // irreducible variation: more evidence would not remove it
}

const (
	LevelLow      = "low"
	LevelModerate = "moderate"
	LevelHigh     = "high"
	LevelUnknown  = "unknown"
)

var UncertaintyLevels = []string{LevelLow, LevelModerate, LevelHigh, LevelUnknown}

var UncertaintyCoverages = []string{"assessed", "partially-assessed", "unassessed"}

var UncertaintyDimensions = []string{"A", "E", "C", "U"}

type UncertaintyDescriptor struct {
	Code    string   `json:"code"`
	Source  string   `json:"source"`
	Kind    string   `json:"kind"`
	Affects []string `json:"affects"`
	// Default level for the code.
	DefaultLevel string `json:"defaultLevel"`
	// Level cannot be set below the registry floor for this code.
	MinLevel    string `json:"minLevel"`
	Description string `json:"description"`
	Mitigation  string `json:"mitigation"`
}

var levelOrder = map[string]int{LevelLow: 0, LevelModerate: 1, LevelHigh: 2, LevelUnknown: 3}

func LevelRank(level string) int {
	rank, ok := levelOrder[level]
	if !ok {
		return -1
	}
	return rank
}

func MaxLevel(a, b string) string {
	if LevelRank(a) >= LevelRank(b) {
		return a
	}
	return b
}

func MinLevel(a, b string) string {
	if LevelRank(a) <= LevelRank(b) {
		return a
	}
	return b
}

func descriptor(code, source, kind string, affects []string, defaultLevel, minLevel, description, mitigation string) UncertaintyDescriptor {
	return UncertaintyDescriptor{Code: code, Source: source, Kind: kind, Affects: affects, DefaultLevel: defaultLevel, MinLevel: minLevel, Description: description, Mitigation: mitigation}
}

// UncertaintyRegistry lists every registered code in registration order.
var UncertaintyRegistry = []UncertaintyDescriptor{
	descriptor("U-RESPONSE-AMBIGUOUS", "response-ambiguity", "epistemic", []string{"A", "U"}, LevelModerate, LevelLow,
		"The answer was given, but the user expressed hesitation or the response is open to more than one reading.",
		"Re-ask the item with an explicit clarification, or record a follow-up observation alongside it rather than overwriting this one."),
	descriptor("U-RESPONSE-MISSING", "instrument-coverage", "epistemic", []string{"A", "U"}, LevelHigh, LevelHigh,
		"The item was skipped or not administered, so the signal is absent rather than negative.",
		"Absence of a response is never treated as a low value; either re-administer or carry the gap forward explicitly."),
	descriptor("U-SELF-REPORT-BIAS", "self-report-bias", "aleatoric", []string{"A", "E", "U"}, LevelModerate, LevelModerate,
		"Self-report instruments are subject to social desirability, impression management, and reference-group effects.",
		"Interpret alongside third-party or behavioural evidence where available; never present a single self-report as a settled finding."),
	descriptor("U-INSTRUMENT-UNVALIDATED", "method-limitation", "epistemic", []string{"E", "U"}, LevelHigh, LevelHigh,
		"The instrument has no documented validation evidence for this population or construct.",
		"Treat the resulting representation as architecture-level information, not as established psychometric measurement (§18)."),
	descriptor("U-INSTRUMENT-COVERAGE-GAP", "instrument-coverage", "epistemic", []string{"A", "C", "U"}, LevelModerate, LevelLow,
		"The instrument does not cover the construct the user asked about, so the evidence is adjacent to the question.",
		"State the coverage gap explicitly on any pathway that would rely on it; gather complementary evidence."),
	descriptor("U-ITEM-WORDING", "item-wording", "epistemic", []string{"A", "U"}, LevelLow, LevelLow,
		"The item may be read differently across populations or translations; wording is a source of variation.",
		"Record the exact item version used, and keep the wording with the observation so interpretation can be revisited."),
	descriptor("U-RECALL-DISTANCE", "memory-recall", "aleatoric", []string{"A", "E", "U"}, LevelModerate, LevelLow,
		"The observation asks the user to recall an event or period some distance in the past.",
		"Prefer recent, dated observations for the same construct; keep this one for longitudinal comparison instead of replacing it."),
	descriptor("U-CONTEXT-UNDECLARED", "context-ambiguity", "epistemic", []string{"C", "U"}, LevelHigh, LevelModerate,
		"Career context is missing or thin, so relevance to any pathway is correspondingly uncertain.",
		"Ask the user for the missing context fields; until then, pathways derived from this observation carry the same flag."),
	descriptor("U-CONTEXT-CONFLICT", "context-ambiguity", "epistemic", []string{"C", "U"}, LevelModerate, LevelModerate,
		"Context fields disagree with each other or with the declared setting of the observation.",
		"Surface both readings to the user; the system does not decide which context applies (§2)."),
	descriptor("U-AGGREGATION-LOSS", "aggregation", "epistemic", []string{"A", "U"}, LevelModerate, LevelLow,
		"Multiple observations were combined, so variation between them is no longer visible in the aggregate.",
		"Keep the contributing observations referenced so the aggregate can be unpacked again."),
	descriptor("U-TRANSFORMATION-LOSS", "transformation-loss", "epistemic", []string{"A", "E", "C", "U"}, LevelModerate, LevelLow,
		"A transformation discarded or abstracted detail from its input representation.",
		"Record the transformation version and its declared losses so the loss is auditable rather than silent."),
	descriptor("U-MODEL-GENERATED", "model-generation", "epistemic", []string{"A", "U"}, LevelHigh, LevelHigh,
		"Text or a factor was produced by a language model rather than observed from the user.",
		"Keep it as a proposal under review; it never counts as an assessment observation on its own (§5)."),
	descriptor("U-QUALITY-UNKNOWN", "method-limitation", "epistemic", []string{"E", "U"}, LevelHigh, LevelModerate,
		"Evidence quality metadata is incomplete, so the strength of the observation cannot be judged.",
		"Complete the quality metadata, or carry the observation with this flag visible wherever it is used."),
	descriptor("U-STALE-OBSERVATION", "stale-observation", "epistemic", []string{"A", "E", "U"}, LevelModerate, LevelLow,
		"The observation is old relative to the decision it is being used for.",
		"Re-assess rather than reinterpret; the historical record stays intact either way (§11)."),
	descriptor("U-TRANSLATION-DRIFT", "translation", "epistemic", []string{"A", "E", "U"}, LevelModerate, LevelModerate,
		"The item or response crossed a language boundary, introducing translation variation.",
		"Record both the source-language and target-language text references with the observation."),
	descriptor("U-METHOD-LIMITED", "method-limitation", "aleatoric", []string{"E", "U"}, LevelModerate, LevelLow,
		"The collection method has inherent limits (single sitting, one setting, small sample).",
		"Treat as one observation among several; gather independent evidence before drawing wider conclusions."),
}

// UncertaintyCodes lists the registered codes in registration order.
func UncertaintyCodes() []string {
	codes := make([]string, len(UncertaintyRegistry))
	for i, d := range UncertaintyRegistry {
		codes[i] = d.Code
	}
	return codes
}

// DescriptorFor returns the registry entry for a code.
func DescriptorFor(code string) (UncertaintyDescriptor, bool) {
	for _, d := range UncertaintyRegistry {
		if d.Code == code {
			return d, true
		}
	}
	return UncertaintyDescriptor{}, false
}

func mustDescriptor(code string) UncertaintyDescriptor {
	d, ok := DescriptorFor(code)
	if !ok {
		panic("unregistered uncertainty code " + code)
	}
	return d
}

type UncertaintyNote struct {
	ID         string   `json:"id"`
	Code       string   `json:"code"`
	Source     string   `json:"source"`
	Kind       string   `json:"kind"`
	Level      string   `json:"level"`
	Detail     string   `json:"detail"`
	Affects    []string `json:"affects"`
	Mitigation string   `json:"mitigation,omitempty"`
	// Who produced the note. Model- and capability-produced notes are marked.
	NotedBy ActorRef `json:"notedBy"`
}

func checkUncertaintyNote(c *checker, path string, note UncertaintyNote) {
	c.enum(joinPath(path, "code"), note.Code, UncertaintyCodes())
	c.enum(joinPath(path, "source"), note.Source, UncertaintySources)
	c.enum(joinPath(path, "kind"), note.Kind, UncertaintyKinds)
	c.enum(joinPath(path, "level"), note.Level, UncertaintyLevels)
	c.str(joinPath(path, "detail"), note.Detail, 1, 500)
	affectsPath := joinPath(path, "affects")
	c.arr(affectsPath, len(note.Affects), 1, -1)
	for i, dimension := range note.Affects {
		c.enum(idx(affectsPath, i), dimension, UncertaintyDimensions)
	}
	c.optStr(joinPath(path, "mitigation"), note.Mitigation, 1, 300)
	checkActor(c, joinPath(path, "notedBy"), note.NotedBy)
}

type UncertaintyProfile struct {
	ProfileVersion string            `json:"profileVersion"`
	Coverage       string            `json:"coverage"`
	Notes          []UncertaintyNote `json:"notes"`
}

func checkUncertaintyProfile(c *checker, path string, profile UncertaintyProfile) {
	c.enum(joinPath(path, "coverage"), profile.Coverage, UncertaintyCoverages)
	notesPath := joinPath(path, "notes")
	if profile.Notes == nil {
		c.add(notesPath, "Invalid input: expected array, received undefined")
		return
	}
	c.arr(notesPath, len(profile.Notes), 0, 12)
	for i, note := range profile.Notes {
		checkUncertaintyNote(c, idx(notesPath, i), note)
	}
}

func ValidateUncertaintyProfile(profile UncertaintyProfile) []Issue {
	c := &checker{}
	checkUncertaintyProfile(c, "", profile)
	return c.result()
}

const UncertaintyProfileVersion = "1.0.0"

// UncertaintySummary is itself lossy and must be labelled as a summary, not
// as the profile (research §13).
type UncertaintySummary struct {
	NoteCount int            `json:"noteCount"`
	MaxLevel  string         `json:"maxLevel"`
	BySource  map[string]int `json:"bySource"`
	ByKind    map[string]int `json:"byKind"`
}

func SummariseUncertainty(profile UncertaintyProfile) UncertaintySummary {
	bySource := map[string]int{}
	byKind := map[string]int{}
	max := LevelLow
	for _, note := range profile.Notes {
		bySource[note.Source]++
		byKind[note.Kind]++
		max = MaxLevel(max, note.Level)
	}
	if len(profile.Notes) == 0 {
		max = LevelUnknown
	}
	return UncertaintySummary{NoteCount: len(profile.Notes), MaxLevel: max, BySource: bySource, ByKind: byKind}
}

// ------------------------------------------------------------------ assess
//
// Derive U_i from what is actually known about an observation. The rules here
// are the project's architecture-level uncertainty policy, not a psychometric
// model. They encode what the repository can defensibly say about how much a
// single observation can support (§4: 1 observation ≠ 1 career conclusion),
// and they are versioned so any change is auditable.

type UncertaintyPolicy struct {
	PolicyVersion string `json:"policyVersion"`
	// A response that exists but reads as hedged/ambiguous raises this note.
	AmbiguityMarkers []string `json:"ambiguityMarkers"`
	// Age, in days, beyond which an observation is flagged as stale.
	StalenessHorizonDays int `json:"stalenessHorizonDays"`
	// Instruments with documented validation for the construct raise no note.
	ValidatedInstruments []string `json:"validatedInstruments"`
	// Minimum number of declared context fields below which context is thin.
	MinContextFields int `json:"minContextFields"`
}

var DefaultUncertaintyPolicy = UncertaintyPolicy{
	PolicyVersion:        UncertaintyProfileVersion,
	AmbiguityMarkers:     []string{"not sure", "unsure", "maybe", "depends", "sometimes", "hard to say", "i guess"},
	StalenessHorizonDays: 730,
	ValidatedInstruments: []string{},
	MinContextFields:     2,
}

type AssessUncertaintyInput struct {
	SignalID              string
	HasResponse           bool
	ResponseText          string
	ResponseKind          string
	InstrumentID          string
	MethodRef             string
	ObservedAtIso         string
	NowIso                string
	ContextFieldCount     int
	ContextConflicts      []string
	QualityKnown          bool
	HasSupportingEvidence bool
}

type AssessUncertaintyOptions struct {
	Policy  *UncertaintyPolicy
	NotedBy *ActorRef
}

func makeNote(signalID, code, level, detail string, notedBy ActorRef, affectsOverride []string) UncertaintyNote {
	d := mustDescriptor(code)
	effective := level
	if LevelRank(level) < LevelRank(d.MinLevel) {
		effective = d.MinLevel
	}
	affects := d.Affects
	if affectsOverride != nil {
		affects = affectsOverride
	}
	return UncertaintyNote{
		ID:         MintID("unc", map[string]any{"signalId": signalID, "code": code, "detail": detail, "level": effective}),
		Code:       code,
		Source:     d.Source,
		Kind:       d.Kind,
		Level:      effective,
		Detail:     detail,
		Affects:    CopyStrings(affects),
		Mitigation: d.Mitigation,
		NotedBy:    notedBy,
	}
}

func daysBetween(fromIso, toIso string) int {
	delta := isoMillis(toIso) - isoMillis(fromIso)
	return int(math.Floor(float64(delta) / 86_400_000))
}

// AssessUncertainty produces the U portion of the 1:3 representation. It never
// returns an empty profile: "assessed" with zero notes is a claim that
// uncertainty was looked for and not found, which is different from
// "unassessed".
func AssessUncertainty(input AssessUncertaintyInput, options AssessUncertaintyOptions) UncertaintyProfile {
	policy := DefaultUncertaintyPolicy
	if options.Policy != nil {
		policy = *options.Policy
	}
	notedBy := SystemActor
	if options.NotedBy != nil {
		notedBy = *options.NotedBy
	}
	notes := []UncertaintyNote{}
	coverage := "assessed"

	if !input.HasResponse {
		coverage = "unassessed"
		notes = append(notes, makeNote(input.SignalID, "U-RESPONSE-MISSING", LevelHigh, "No response was recorded for this observation.", notedBy, nil))
	} else {
		notes = append(notes, makeNote(input.SignalID, "U-SELF-REPORT-BIAS", LevelModerate,
			"Self-report observation: the reading reflects the user’s account of themselves at one moment.", notedBy, nil))
	}

	text := strings.ToLower(input.ResponseText)
	if text != "" {
		for _, marker := range policy.AmbiguityMarkers {
			if strings.Contains(text, marker) {
				notes = append(notes, makeNote(input.SignalID, "U-RESPONSE-AMBIGUOUS", LevelModerate,
					fmt.Sprintf("The response contains a hedge (%q), so it supports more than one reading.", marker), notedBy, nil))
				break
			}
		}
	}

	if !Contains(policy.ValidatedInstruments, input.InstrumentID) {
		notes = append(notes, makeNote(input.SignalID, "U-INSTRUMENT-UNVALIDATED", LevelHigh,
			fmt.Sprintf("Instrument %q has no validation reference recorded in this deployment; measurements from it are architectural information, not validated psychometrics.", input.InstrumentID),
			notedBy, nil))
	}

	if input.ContextFieldCount < policy.MinContextFields {
		level := LevelModerate
		if input.ContextFieldCount == 0 {
			level = LevelHigh
		}
		notes = append(notes, makeNote(input.SignalID, "U-CONTEXT-UNDECLARED", level,
			fmt.Sprintf("Only %d context field(s) declared; below the policy minimum of %d.", input.ContextFieldCount, policy.MinContextFields),
			notedBy, nil))
	}

	for _, conflict := range input.ContextConflicts {
		notes = append(notes, makeNote(input.SignalID, "U-CONTEXT-CONFLICT", LevelModerate, conflict, notedBy, nil))
	}

	if !input.QualityKnown {
		notes = append(notes, makeNote(input.SignalID, "U-QUALITY-UNKNOWN", LevelHigh, "Evidence quality metadata is incomplete for this observation.", notedBy, nil))
	}

	if !input.HasSupportingEvidence {
		notes = append(notes, makeNote(input.SignalID, "U-METHOD-LIMITED", LevelModerate,
			fmt.Sprintf("Collection method %q stands alone for this construct; no independent evidence is referenced.", input.MethodRef),
			notedBy, nil))
	}

	age := daysBetween(input.ObservedAtIso, input.NowIso)
	if age > policy.StalenessHorizonDays {
		notes = append(notes, makeNote(input.SignalID, "U-STALE-OBSERVATION", LevelModerate,
			fmt.Sprintf("Observation is %d days old; the policy horizon is %d days.", age, policy.StalenessHorizonDays),
			notedBy, nil))
	}

	profile := UncertaintyProfile{ProfileVersion: policy.PolicyVersion, Coverage: coverage, Notes: notes}
	mustValid("uncertainty profile", ValidateUncertaintyProfile(profile))
	return profile
}

func EnsureUnassessedProfile(signalID, detail string, notedBy *ActorRef) UncertaintyProfile {
	by := SystemActor
	if notedBy != nil {
		by = *notedBy
	}
	profile := UncertaintyProfile{
		ProfileVersion: UncertaintyProfileVersion,
		Coverage:       "unassessed",
		Notes:          []UncertaintyNote{makeNote(signalID, "U-QUALITY-UNKNOWN", LevelUnknown, detail, by, nil)},
	}
	mustValid("uncertainty profile", ValidateUncertaintyProfile(profile))
	return profile
}

// MergeUncertainty preserves U through transformations (§21): merging unions
// notes and takes the weakest coverage; nothing is dropped.
func MergeUncertainty(profiles []UncertaintyProfile, external []UncertaintyNote, notedBy *ActorRef) UncertaintyProfile {
	by := SystemActor
	if notedBy != nil {
		by = *notedBy
	}
	notes := []UncertaintyNote{}
	seen := map[string]bool{}
	coverage := "assessed"
	profileVersion := UncertaintyProfileVersion
	coverageRank := map[string]int{"assessed": 0, "partially-assessed": 1, "unassessed": 2}

	for _, profile := range profiles {
		if coverageRank[profile.Coverage] >= coverageRank[coverage] {
			coverage = profile.Coverage
		}
		profileVersion = profile.ProfileVersion
		for _, existing := range profile.Notes {
			key := existing.Code + "|" + existing.Detail
			if seen[key] {
				continue
			}
			seen[key] = true
			notes = append(notes, existing)
		}
	}
	for _, note := range external {
		key := note.Code + "|" + note.Detail
		if seen[key] {
			continue
		}
		seen[key] = true
		notes = append(notes, note)
	}
	if len(notes) == 0 {
		notes = append(notes, makeNote(MintID("sig", map[string]any{"profiles": len(profiles)}), "U-METHOD-LIMITED", LevelLow,
			"No specific uncertainty was identified for the merged inputs.", by, nil))
	}
	if len(notes) > 12 {
		notes = notes[:12]
	}
	profile := UncertaintyProfile{ProfileVersion: profileVersion, Coverage: coverage, Notes: notes}
	mustValid("uncertainty profile", ValidateUncertaintyProfile(profile))
	return profile
}

func UncertaintyMaximumLevel(profile UncertaintyProfile) string {
	level := LevelLow
	for _, note := range profile.Notes {
		level = MaxLevel(level, note.Level)
	}
	if len(profile.Notes) == 0 {
		return LevelUnknown
	}
	return level
}

func IsHighUncertainty(profile UncertaintyProfile) bool {
	return LevelRank(UncertaintyMaximumLevel(profile)) >= LevelRank(LevelHigh)
}

// mustValid mirrors a schema parse that throws: it is used where the inputs
// are produced by the domain itself, so a failure is a programming error.
func mustValid(what string, issues []Issue) {
	if len(issues) > 0 {
		panic(fmt.Sprintf("%s failed validation: %s", what, strings.Join(IssueStrings(issues), "; ")))
	}
}
