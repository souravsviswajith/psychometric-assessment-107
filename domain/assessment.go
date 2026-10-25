package domain

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// Instrument vocabulary and the conversion of a raw response into an
// interpretable value.
//
// Design rule (§4): a raw response becomes an interpreted observation, never
// a score. Position is a normalized reading of one response against a declared
// interpretation policy; it is not a career quantity, and it is never summed
// into a universal career score.

var ConstructKinds = []string{
	"interest",
	"value",
	"skill-self-report",
	"work-style",
	"preference",
	"motivation",
	"experience",
	"constraint",
	"context-condition",
}

// ConstructRef references the project construct vocabulary — a project
// vocabulary, not a validated psychometric taxonomy (§13, §18).
type ConstructRef struct {
	ConstructID     string `json:"constructId"`
	Label           string `json:"label"`
	Kind            string `json:"kind"`
	TaxonomyRef     string `json:"taxonomyRef"`
	TaxonomyVersion string `json:"taxonomyVersion"`
}

var constructIDPattern = regexp.MustCompile(`^[a-z][a-z0-9.-]{1,63}$`)

func checkConstructRef(c *checker, path string, ref ConstructRef) {
	c.match(joinPath(path, "constructId"), ref.ConstructID, constructIDPattern, "construct ids are dotted lowercase ids")
	c.str(joinPath(path, "label"), ref.Label, 1, 80)
	c.enum(joinPath(path, "kind"), ref.Kind, ConstructKinds)
	c.str(joinPath(path, "taxonomyRef"), ref.TaxonomyRef, 1, 120)
	checkSemver(c, joinPath(path, "taxonomyVersion"), ref.TaxonomyVersion)
}

var ResponseShapes = []string{
	"likert-5",
	"likert-7",
	"frequency-5",
	"single-select",
	"multi-select",
	"open-text",
	"numeric-count",
}

var OrdinalShapes = []string{"likert-5", "likert-7", "frequency-5", "numeric-count"}

type InterpretationPolicy struct {
	// Stable policy reference, e.g. "policy.likert-ascending.v1".
	ID      string `json:"id"`
	Version string `json:"version"`
	// Which end of the presented scale corresponds to more of the construct.
	// Recorded with every observation so the reading can be recomputed (§11).
	Direction string `json:"direction"`
	Statement string `json:"statement"`
}

func checkPolicy(c *checker, path string, policy InterpretationPolicy) {
	c.str(joinPath(path, "id"), policy.ID, 1, 120)
	checkSemver(c, joinPath(path, "version"), policy.Version)
	c.enum(joinPath(path, "direction"), policy.Direction, []string{"ascending", "descending"})
	c.str(joinPath(path, "statement"), policy.Statement, 1, 300)
}

type NumericScale struct {
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
	Unit string  `json:"unit"`
}

// ResponseInterpretation is the interpreted response. Each variant (ordinal,
// categorical, text) carries the raw material it was derived from, so no
// observation is lost by interpretation (§13, experiment A). Only the members
// of the active kind are set; the others are omitted from serialisation.
type ResponseInterpretation struct {
	Kind   string               `json:"kind"`
	Shape  string               `json:"shape"`
	Policy InterpretationPolicy `json:"policy"`

	// ordinal: 0-based index of the selected option in the presented order.
	RawIndex *int `json:"rawIndex,omitempty"`
	// ordinal: presented option labels, in presented order.
	PresentedLabels []string `json:"presentedLabels,omitempty"`
	// ordinal: amount of the construct asserted by this response, in [0,1].
	Position *float64      `json:"position,omitempty"`
	RawValue *float64      `json:"rawValue,omitempty"`
	Scale    *NumericScale `json:"scale,omitempty"`

	// categorical
	SelectedOptionIDs []string `json:"selectedOptionIds,omitempty"`
	SelectedLabels    []string `json:"selectedLabels,omitempty"`

	// text
	Text      string `json:"text,omitempty"`
	CharCount int    `json:"charCount,omitempty"`
}

func (r ResponseInterpretation) PositionValue() float64 {
	if r.Position == nil {
		return 0
	}
	return *r.Position
}

func (r ResponseInterpretation) RawIndexValue() int {
	if r.RawIndex == nil {
		return 0
	}
	return *r.RawIndex
}

// PresentedLabel returns the label at rawIndex, or "—" when it is absent.
func (r ResponseInterpretation) PresentedLabel() string {
	i := r.RawIndexValue()
	if i >= 0 && i < len(r.PresentedLabels) {
		return r.PresentedLabels[i]
	}
	return "—"
}

func checkInterpretation(c *checker, path string, r ResponseInterpretation) {
	switch r.Kind {
	case "ordinal":
		c.enum(joinPath(path, "shape"), r.Shape, OrdinalShapes)
		checkPolicy(c, joinPath(path, "policy"), r.Policy)
		if r.RawIndex == nil {
			c.add(joinPath(path, "rawIndex"), "Invalid input: expected number, received undefined")
		} else if *r.RawIndex < 0 {
			c.add(joinPath(path, "rawIndex"), "Too small: expected number to be >=0")
		}
		c.strings(joinPath(path, "presentedLabels"), r.PresentedLabels, 1, 120, 2, 9)
		if r.Position == nil {
			c.add(joinPath(path, "position"), "Invalid input: expected number, received undefined")
		} else if *r.Position < 0 || *r.Position > 1 || math.IsNaN(*r.Position) {
			c.add(joinPath(path, "position"), "position must be within [0, 1]")
		}
		if r.RawValue != nil && (math.IsNaN(*r.RawValue) || math.IsInf(*r.RawValue, 0)) {
			c.add(joinPath(path, "rawValue"), "Invalid input: expected number, received NaN")
		}
		if r.RawIndex != nil && *r.RawIndex > len(r.PresentedLabels)-1 {
			c.add(joinPath(path, "rawIndex"), "rawIndex must address a presented option")
		}
		if r.Shape == "numeric-count" && r.RawValue == nil {
			c.add(joinPath(path, "rawValue"), "numeric-count responses require rawValue")
		}
		if r.Shape == "numeric-count" && r.Scale == nil {
			c.add(joinPath(path, "scale"), "numeric-count responses require a declared scale")
		}
	case "categorical":
		c.enum(joinPath(path, "shape"), r.Shape, []string{"single-select", "multi-select"})
		checkPolicy(c, joinPath(path, "policy"), r.Policy)
		c.strings(joinPath(path, "selectedOptionIds"), r.SelectedOptionIDs, 1, 80, 1, 8)
		c.strings(joinPath(path, "selectedLabels"), r.SelectedLabels, 1, 120, 1, 8)
	case "text":
		if r.Shape != "open-text" {
			c.add(joinPath(path, "shape"), `Invalid input: expected "open-text"`)
		}
		checkPolicy(c, joinPath(path, "policy"), r.Policy)
		c.str(joinPath(path, "text"), r.Text, 1, 4000)
		c.intRange(joinPath(path, "charCount"), r.CharCount, 1, 4000, true)
	default:
		c.add(joinPath(path, "kind"), `Invalid input: expected one of "ordinal"|"categorical"|"text"`)
	}
}

// ValidateInterpretation checks an interpreted response against its schema.
func ValidateInterpretation(r ResponseInterpretation) []Issue {
	c := &checker{}
	checkInterpretation(c, "", r)
	return c.result()
}

// InstrumentRef: version is mandatory — a result must be reproducible from
// input + version set (§11), and an instrument that changes wording is a
// different instrument.
type InstrumentRef struct {
	InstrumentID      string `json:"instrumentId"`
	InstrumentVersion string `json:"instrumentVersion"`
	ItemRef           string `json:"itemRef"`
	ItemVersion       string `json:"itemVersion"`
	// The exact wording presented. Kept with the observation, never re-derived.
	PromptText    string `json:"promptText"`
	ResponseShape string `json:"responseShape"`
	// True when the instrument has a documented validation reference.
	Validated     bool   `json:"validated"`
	ValidationRef string `json:"validationRef,omitempty"`
}

func checkInstrumentRef(c *checker, path string, ref InstrumentRef) {
	c.str(joinPath(path, "instrumentId"), ref.InstrumentID, 1, 120)
	checkSemver(c, joinPath(path, "instrumentVersion"), ref.InstrumentVersion)
	c.str(joinPath(path, "itemRef"), ref.ItemRef, 1, 120)
	checkSemver(c, joinPath(path, "itemVersion"), ref.ItemVersion)
	c.str(joinPath(path, "promptText"), ref.PromptText, 1, 1000)
	c.enum(joinPath(path, "responseShape"), ref.ResponseShape, ResponseShapes)
	c.optStr(joinPath(path, "validationRef"), ref.ValidationRef, 1, 200)
}

// AssessmentFacet is the A facet of 1:3 — what was observed, measured,
// answered, or assessed. It carries the signal identity; it is a view of one
// observation, not a new record.
type AssessmentFacet struct {
	// SignalId(A_i) — identity is carried, never re-minted (§6).
	ID            string                 `json:"id"`
	SignalID      string                 `json:"signalId"`
	SchemaVersion string                 `json:"schemaVersion"`
	SubjectID     string                 `json:"subjectId"`
	Instrument    InstrumentRef          `json:"instrument"`
	Constructs    []ConstructRef         `json:"constructs"`
	Response      ResponseInterpretation `json:"response"`
	ObservedAt    string                 `json:"observedAt"`
	// Where the observation was produced.
	Setting string `json:"setting"`
}

func checkAssessmentFacet(c *checker, path string, a AssessmentFacet) {
	SignalIDKind.check(c, joinPath(path, "id"), a.ID)
	SignalIDKind.check(c, joinPath(path, "signalId"), a.SignalID)
	checkSemver(c, joinPath(path, "schemaVersion"), a.SchemaVersion)
	SubjectIDKind.check(c, joinPath(path, "subjectId"), a.SubjectID)
	checkInstrumentRef(c, joinPath(path, "instrument"), a.Instrument)
	c.arr(joinPath(path, "constructs"), len(a.Constructs), 1, 4)
	for i, construct := range a.Constructs {
		checkConstructRef(c, idx(joinPath(path, "constructs"), i), construct)
	}
	checkInterpretation(c, joinPath(path, "response"), a.Response)
	checkIso(c, joinPath(path, "observedAt"), a.ObservedAt)
	c.str(joinPath(path, "setting"), a.Setting, 1, 120)
}

const AssessmentFacetSchemaVersion = "1.0.0"

// Instrument contracts. The instrument catalogue lives beside its items, but
// the shape it must satisfy is a domain contract.

type InstrumentOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type InstrumentItem struct {
	InstrumentID      string               `json:"instrumentId"`
	InstrumentVersion string               `json:"instrumentVersion"`
	ItemRef           string               `json:"itemRef"`
	ItemVersion       string               `json:"itemVersion"`
	PromptText        string               `json:"promptText"`
	ResponseShape     string               `json:"responseShape"`
	Options           []InstrumentOption   `json:"options"`
	Policy            InterpretationPolicy `json:"policy"`
	Scale             *NumericScale        `json:"scale,omitempty"`
	Constructs        []ConstructRef       `json:"constructs"`
	Validated         bool                 `json:"validated"`
	ValidationRef     string               `json:"validationRef,omitempty"`
	// What this item can and cannot tell us — carried into the evidence method.
	Limitations []string `json:"limitations"`
}

type InstrumentDefinition struct {
	InstrumentID      string           `json:"instrumentId"`
	InstrumentVersion string           `json:"instrumentVersion"`
	Title             string           `json:"title"`
	Status            string           `json:"status"`
	Items             []InstrumentItem `json:"items"`
}

// ------------------------------------------------------------ interpretation
//
// This is the only place a raw answer becomes interpretable. It never produces
// a score, a rank, or a career quantity: it produces a normalized reading with
// the policy that produced it recorded alongside (§11).

// RawResponse is what the user actually answered. Kind selects the variant:
// option{optionId}, options{optionIds}, text{text}, number{value}.
type RawResponse struct {
	Kind      string   `json:"kind"`
	OptionID  string   `json:"optionId,omitempty"`
	OptionIDs []string `json:"optionIds,omitempty"`
	Text      string   `json:"text,omitempty"`
	Value     *float64 `json:"value,omitempty"`
}

func checkRawResponse(c *checker, path string, raw RawResponse) {
	switch raw.Kind {
	case "option":
		c.str(joinPath(path, "optionId"), raw.OptionID, 1, 80)
	case "options":
		c.strings(joinPath(path, "optionIds"), raw.OptionIDs, 1, 80, 1, 8)
	case "text":
		c.str(joinPath(path, "text"), raw.Text, 1, 4000)
	case "number":
		if raw.Value == nil {
			c.add(joinPath(path, "value"), "Invalid input: expected number, received undefined")
		} else if math.IsNaN(*raw.Value) || math.IsInf(*raw.Value, 0) {
			c.add(joinPath(path, "value"), "Invalid input: expected number, received NaN")
		}
	default:
		c.add(joinPath(path, "kind"), `Invalid input: expected one of "option"|"options"|"text"|"number"`)
	}
}

func normalizePosition(index, optionCount int, descending bool) float64 {
	if optionCount <= 1 {
		return 0
	}
	ratio := float64(index) / float64(optionCount-1)
	clamped := math.Min(1, math.Max(0, ratio))
	if descending {
		return 1 - clamped
	}
	return clamped
}

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }

// InterpretResponse turns a raw response into an interpreted observation, or
// returns the issues that make it uninterpretable.
func InterpretResponse(item InstrumentItem, raw RawResponse) (ResponseInterpretation, []string) {
	descending := item.Policy.Direction == "descending"

	finish := func(r ResponseInterpretation) (ResponseInterpretation, []string) {
		if issues := ValidateInterpretation(r); len(issues) > 0 {
			return ResponseInterpretation{}, IssueMessages(issues)
		}
		return r, nil
	}

	if item.ResponseShape == "open-text" {
		if raw.Kind != "text" {
			return ResponseInterpretation{}, []string{fmt.Sprintf("item %s expects a text response, received %q", item.ItemRef, raw.Kind)}
		}
		return finish(ResponseInterpretation{
			Kind:      "text",
			Shape:     "open-text",
			Policy:    item.Policy,
			Text:      raw.Text,
			CharCount: JSLength(raw.Text),
		})
	}

	if item.ResponseShape == "single-select" || item.ResponseShape == "multi-select" {
		var optionIDs []string
		switch raw.Kind {
		case "option":
			optionIDs = []string{raw.OptionID}
		case "options":
			optionIDs = raw.OptionIDs
		}
		if len(optionIDs) == 0 {
			return ResponseInterpretation{}, []string{fmt.Sprintf("item %s expects a selection, received %q", item.ItemRef, raw.Kind)}
		}
		var issues []string
		if item.ResponseShape == "single-select" && len(optionIDs) != 1 {
			issues = append(issues, fmt.Sprintf("item %s is single-select but received %d options", item.ItemRef, len(optionIDs)))
		}
		labels := []string{}
		for _, optionID := range optionIDs {
			found := false
			for _, option := range item.Options {
				if option.ID == optionID {
					labels = append(labels, option.Label)
					found = true
					break
				}
			}
			if !found {
				issues = append(issues, fmt.Sprintf("item %s has no option %q", item.ItemRef, optionID))
			}
		}
		if len(issues) > 0 {
			return ResponseInterpretation{}, issues
		}
		return finish(ResponseInterpretation{
			Kind:              "categorical",
			Shape:             item.ResponseShape,
			Policy:            item.Policy,
			SelectedOptionIDs: CopyStrings(optionIDs),
			SelectedLabels:    labels,
		})
	}

	if item.ResponseShape == "numeric-count" {
		if raw.Kind != "number" || raw.Value == nil {
			return ResponseInterpretation{}, []string{fmt.Sprintf("item %s expects a numeric response, received %q", item.ItemRef, raw.Kind)}
		}
		if item.Scale == nil {
			return ResponseInterpretation{}, []string{fmt.Sprintf("item %s is numeric-count but declares no scale", item.ItemRef)}
		}
		scale := *item.Scale
		span := scale.Max - scale.Min
		ratio := 0.0
		if span != 0 {
			ratio = (*raw.Value - scale.Min) / span
		}
		clamped := math.Min(1, math.Max(0, ratio))
		position := clamped
		if descending {
			position = 1 - clamped
		}
		return finish(ResponseInterpretation{
			Kind:            "ordinal",
			Shape:           "numeric-count",
			Policy:          item.Policy,
			RawIndex:        intPtr(0),
			PresentedLabels: []string{JSNumber(scale.Min), JSNumber(scale.Max)},
			Position:        floatPtr(position),
			RawValue:        floatPtr(*raw.Value),
			Scale:           &scale,
		})
	}

	// Ordinal shapes: likert-5, likert-7, frequency-5.
	if raw.Kind != "option" {
		return ResponseInterpretation{}, []string{fmt.Sprintf("item %s expects an option response, received %q", item.ItemRef, raw.Kind)}
	}
	index := -1
	for i, option := range item.Options {
		if option.ID == raw.OptionID {
			index = i
			break
		}
	}
	if index < 0 {
		return ResponseInterpretation{}, []string{fmt.Sprintf("item %s has no option %q", item.ItemRef, raw.OptionID)}
	}
	labels := make([]string, len(item.Options))
	for i, option := range item.Options {
		labels[i] = option.Label
	}
	return finish(ResponseInterpretation{
		Kind:            "ordinal",
		Shape:           item.ResponseShape,
		Policy:          item.Policy,
		RawIndex:        intPtr(index),
		PresentedLabels: labels,
		Position:        floatPtr(normalizePosition(index, len(item.Options), descending)),
	})
}

// DescribeInterpretation is a concise, user-facing rendering.
func DescribeInterpretation(r ResponseInterpretation) string {
	switch r.Kind {
	case "ordinal":
		return fmt.Sprintf("%s (position %s, %s)", r.PresentedLabel(), JSFixed(r.PositionValue(), 2), r.Shape)
	case "categorical":
		return fmt.Sprintf("%s (%s)", strings.Join(r.SelectedLabels, ", "), r.Shape)
	case "text":
		if JSLength(r.Text) > 120 {
			return JSSlice(r.Text, 0, 117) + "…"
		}
		return r.Text
	}
	return ""
}

// ------------------------------------------------------------------ taxonomy
//
// Status: PROJECT VOCABULARY, NOT A VALIDATED PSYCHOMETRIC TAXONOMY. It exists
// so that observations, evidence, contexts, and pathway requirements can
// reference the same construct identifiers.

const (
	VocabularyID      = "career107.construct-vocabulary"
	VocabularyVersion = "0.1.0"
)

type ConstructDefinition struct {
	ConstructRef
	Note string `json:"note"`
}

func construct(id, label, kind, note string) ConstructDefinition {
	return ConstructDefinition{
		ConstructRef: ConstructRef{
			ConstructID:     id,
			Label:           label,
			Kind:            kind,
			TaxonomyRef:     VocabularyID + "@" + VocabularyVersion,
			TaxonomyVersion: VocabularyVersion,
		},
		Note: note,
	}
}

// Constructs is the vocabulary in declaration order.
var Constructs = []ConstructDefinition{
	construct("interest.analysis", "Analytical activity interest", "interest", "Reported interest in examining information, patterns, or systems (self-report)."),
	construct("interest.creation", "Creative production interest", "interest", "Reported interest in producing artifacts, designs, or material."),
	construct("interest.social", "Social interaction interest", "interest", "Reported interest in work that centres on other people."),
	construct("interest.organisation", "Organising and coordinating interest", "interest", "Reported interest in planning, sequencing, and coordinating work."),
	construct("interest.building", "Hands-on building interest", "interest", "Reported interest in physical construction, tools, or operating equipment."),
	construct("interest.explanation", "Explanation and teaching interest", "interest", "Reported interest in explaining, teaching, or writing for others."),
	construct("value.autonomy", "Autonomy", "value", "Declared importance of directing one’s own work."),
	construct("value.impact", "Impact on others", "value", "Declared importance of visible consequence for people or society."),
	construct("value.stability", "Stability and predictability", "value", "Declared importance of continuity and predictable conditions."),
	construct("value.growth", "Skill growth", "value", "Declared importance of continued learning and capability increase."),
	construct("value.compensation-boundary", "Compensation adequacy boundary", "value", "Declared minimum sufficiency of compensation; recorded as a boundary, not as an amount of income."),
	construct("style.structure-preference", "Structured vs. emergent work", "work-style", "Declared preference for defined versus open-ended work."),
	construct("style.collaboration", "Collaborative vs. independent work", "work-style", "Declared preference for working with others versus alone."),
	construct("style.depth", "Depth vs. breadth", "work-style", "Declared preference for deep specialisation versus broad involvement."),
	construct("skill.writing", "Written communication (self-report)", "skill-self-report", "Self-reported capability in written communication; unverified, self-report only."),
	construct("skill.quantitative", "Quantitative reasoning (self-report)", "skill-self-report", "Self-reported capability with numbers and quantitative reasoning."),
	construct("skill.tooling", "Software tooling (self-report)", "skill-self-report", "Self-reported capability with software tools; evidence is context-bound."),
	construct("skill.facilitation", "Facilitation (self-report)", "skill-self-report", "Self-reported capability in guiding groups and conversations."),
	construct("experience.applied-projects", "Applied project experience", "experience", "Declared experience of carrying work through to an outcome."),
	construct("constraint.schedule", "Schedule constraint", "constraint", "Declared constraint on when work can happen (e.g. caring responsibilities expressed as availability windows)."),
	construct("constraint.geography", "Geographic constraint", "constraint", "Declared constraint on where work can happen (expressed as mobility/commute band, not as residence)."),
}

func ConstructIDs() []string {
	ids := make([]string, len(Constructs))
	for i, definition := range Constructs {
		ids[i] = definition.ConstructID
	}
	return ids
}

// LookupConstructRef returns the reference for a known construct.
func LookupConstructRef(constructID string) (ConstructRef, bool) {
	for _, definition := range Constructs {
		if definition.ConstructID == constructID {
			return definition.ConstructRef, true
		}
	}
	return ConstructRef{}, false
}

func mustConstructRef(constructID string) ConstructRef {
	ref, ok := LookupConstructRef(constructID)
	if !ok {
		panic("unknown construct " + constructID)
	}
	c := &checker{}
	checkConstructRef(c, "", ref)
	if len(c.result()) > 0 {
		panic("invalid construct " + constructID)
	}
	return ref
}

func IsKnownConstruct(constructID string) bool {
	_, ok := LookupConstructRef(constructID)
	return ok
}
