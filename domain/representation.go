package domain

import (
	"fmt"
	"strings"
)

// The canonical 1:3 infrastructure representation.
//
//	x_i → (A_i, E_i, C_i) | U_i
//
// A is what was observed; E is why it exists and what supports it; C is the
// context in which it may legitimately be relevant; U is cross-cutting
// uncertainty metadata attached to all three. U is not a fourth semantic
// dimension (§3) — it is a qualifier over the triple.
//
// Identity invariant (§6): A.id = E.id = C.id = SignalId.

const RepresentationSchemaVersion = "1.3.0"

type EvidenceFacet struct {
	// E_i identity — equal to the signal identity (§6).
	ID                string           `json:"id"`
	PrimaryEvidenceID string           `json:"primaryEvidenceId"`
	Bundle            []EvidenceRecord `json:"bundle"`
}

type Representation1to3 struct {
	ID            string          `json:"id"`
	SchemaVersion string          `json:"schemaVersion"`
	SignalID      string          `json:"signalId"`
	SubjectID     string          `json:"subjectId"`
	A             AssessmentFacet `json:"A"`
	E             EvidenceFacet   `json:"E"`
	C             CareerContext   `json:"C"`
	// Cross-cutting uncertainty over (A, E, C).
	U         UncertaintyProfile `json:"U"`
	CreatedAt string             `json:"createdAt"`
	CreatedBy ActorRef           `json:"createdBy"`
	// Transformation that produced this representation from its signal.
	TransformationRef string `json:"transformationRef"`
}

func ValidateRepresentation(r Representation1to3) []Issue {
	c := &checker{}
	RepresentationIDKind.check(c, "id", r.ID)
	checkSemver(c, "schemaVersion", r.SchemaVersion)
	SignalIDKind.check(c, "signalId", r.SignalID)
	SubjectIDKind.check(c, "subjectId", r.SubjectID)
	checkAssessmentFacet(c, "A", r.A)
	SignalIDKind.check(c, "E.id", r.E.ID)
	c.str("E.primaryEvidenceId", r.E.PrimaryEvidenceID, 1, -1)
	checkEvidenceBundle(c, "E.bundle", r.E.Bundle)
	checkContext(c, "C", r.C)
	checkUncertaintyProfile(c, "U", r.U)
	checkIso(c, "createdAt", r.CreatedAt)
	checkActor(c, "createdBy", r.CreatedBy)
	c.str("transformationRef", r.TransformationRef, 1, 120)

	if r.A.ID != r.SignalID {
		c.add("A.id", "A.id must equal the originating signal id")
	}
	if r.E.ID != r.SignalID {
		c.add("E.id", "E.id must equal the originating signal id")
	}
	if r.C.ID != r.SignalID {
		c.add("C.id", "C.id must equal the originating signal id")
	}
	if r.A.SignalID != r.SignalID {
		c.add("A.signalId", "A must reference its originating signal")
	}
	resolved := false
	for _, record := range r.E.Bundle {
		if record.ID == r.E.PrimaryEvidenceID {
			resolved = true
			break
		}
	}
	if !resolved {
		c.add("E.primaryEvidenceId", "primary evidence reference must resolve inside the bundle")
	}
	if r.A.SubjectID != r.SubjectID {
		c.add("A.subjectId", "A and the representation must share a subject")
	}
	if r.U.Coverage == "assessed" && len(r.U.Notes) == 0 {
		c.add("U", `coverage "assessed" requires at least one uncertainty note; use "unassessed" when uncertainty was not examined`)
	}
	return c.result()
}

// PrimaryEvidence resolves the primary evidence reference inside the bundle.
func (r Representation1to3) PrimaryEvidence() (EvidenceRecord, bool) {
	for _, record := range r.E.Bundle {
		if record.ID == r.E.PrimaryEvidenceID {
			return record, true
		}
	}
	return EvidenceRecord{}, false
}

// --------------------------------------------------------------- decompose
//
// S_0 signal → S_1 representation (1:3). This is a transformation, not a
// rewrite: the signal is not consumed, and the resulting representation
// carries the signal identity so that A, E, and C can never drift apart (§6).

const (
	DecomposeTransformationID      = "transformation.decompose-1to3"
	DecomposeTransformationVersion = "1.0.0"
)

type DecomposeParams struct {
	Signal    Signal
	CreatedAt string
	CreatedBy ActorRef
	// Overrides the recorded transformation reference (used by research fixtures).
	TransformationRef string
}

func settingOf(s Signal) string {
	if declared, ok := FindContextField(s.Context, "setting"); ok {
		if value, isString := declared.Value.AsString(); isString {
			return value
		}
	}
	if stage, ok := FindContextField(s.Context, "careerStage"); ok {
		if value, isString := stage.Value.AsString(); isString {
			return value + " (unspecified setting)"
		}
	}
	return "unspecified"
}

// ConstructRefsFor resolves item → constructs. Kept in one place so the
// mapping is versioned with the instrument rather than embedded inside
// transformation code.
func ConstructRefsFor(itemRef string) []ConstructRef {
	item, ok := FindItem(itemRef)
	if !ok {
		return nil
	}
	return item.Constructs
}

func DecomposeSignal(params DecomposeParams) (Representation1to3, []Rejection) {
	s := params.Signal
	constructs := ConstructRefsFor(s.Instrument.ItemRef)
	if len(constructs) == 0 {
		return Representation1to3{}, []Rejection{NewRejection("S1.REPRESENTATION.CONSTRUCTS",
			fmt.Sprintf("item %q resolves to no assessment construct; the observation cannot be interpreted", s.Instrument.ItemRef),
			"A.constructs")}
	}

	observedAt := s.CapturedAt
	if len(s.Evidence) > 0 {
		observedAt = s.Evidence[0].Temporal.ObservedAt
	}
	a := AssessmentFacet{
		ID:            s.ID,
		SignalID:      s.ID,
		SchemaVersion: AssessmentFacetSchemaVersion,
		SubjectID:     s.SubjectID,
		Instrument:    s.Instrument,
		Constructs:    append([]ConstructRef{}, constructs...),
		Response:      s.Response,
		ObservedAt:    observedAt,
		Setting:       settingOf(s),
	}

	transformationRef := params.TransformationRef
	if transformationRef == "" {
		transformationRef = DecomposeTransformationID
	}
	primaryID := s.ID
	if primary, ok := PrimaryOf(s.Evidence); ok {
		primaryID = primary.ID
	}

	candidate := Representation1to3{
		ID: MintID("r13", map[string]any{
			"signalId":             s.ID,
			"representationSchema": RepresentationSchemaVersion,
			"transformation":       transformationRef,
		}),
		SchemaVersion: RepresentationSchemaVersion,
		SignalID:      s.ID,
		SubjectID:     s.SubjectID,
		A:             a,
		E: EvidenceFacet{
			ID:                s.ID,
			PrimaryEvidenceID: primaryID,
			Bundle:            s.Evidence,
		},
		C:                 s.Context,
		U:                 s.Uncertainty,
		CreatedAt:         params.CreatedAt,
		CreatedBy:         params.CreatedBy,
		TransformationRef: transformationRef + "@" + DecomposeTransformationVersion,
	}

	if issues := ValidateRepresentation(candidate); len(issues) > 0 {
		return Representation1to3{}, IssuesToRejections("S1.REPRESENTATION.SHAPE", issues, "representation")
	}
	return candidate, nil
}

// IdentityPreserved is true when the representation preserves the identity
// invariant (§6/§21).
func IdentityPreserved(r Representation1to3) bool {
	return r.A.ID == r.E.ID && r.E.ID == r.C.ID && r.C.ID == r.SignalID
}

// ------------------------------------------------------------------ render
//
// Render a representation for human review. Used by the web projection and
// by research experiment B (provenance recovery). The renderer is deliberately
// the only path from representation to prose, so that "what a reviewer can
// see" is a measurable property of the system rather than an accident of UI
// code.

type RenderedRow struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Detail string `json:"detail,omitempty"`
	// Where the value comes from, in reviewer-readable form.
	Provenance  string `json:"provenance,omitempty"`
	Uncertainty string `json:"uncertainty,omitempty"`
}

type RenderedSection struct {
	Key   string        `json:"key"`
	Title string        `json:"title"`
	Rows  []RenderedRow `json:"rows"`
}

type RenderedRepresentation struct {
	Headline string            `json:"headline"`
	Subhead  string            `json:"subhead"`
	Sections []RenderedSection `json:"sections"`
	// Facets excluded by the render options; used to measure provenance recovery.
	Restricted []string `json:"restricted"`
}

type RenderOptions struct {
	// Experiment B: restrict E and measure what a reviewer can still recover.
	RestrictEvidence    bool
	RestrictContext     bool
	RestrictUncertainty bool
}

func EvidenceProvenance(r Representation1to3) string {
	primary, ok := r.PrimaryEvidence()
	if !ok {
		return "primary evidence reference unresolved"
	}
	return fmt.Sprintf("%s · %s@%s · observed %s", primary.Provenance.SourceRef, primary.Method.Ref, primary.Method.Version, primary.Temporal.ObservedAt)
}

func RenderRepresentation(r Representation1to3, options RenderOptions) RenderedRepresentation {
	restricted := []string{}
	sections := []RenderedSection{}

	constructs := make([]string, len(r.A.Constructs))
	labels := make([]string, len(r.A.Constructs))
	for i, construct := range r.A.Constructs {
		constructs[i] = fmt.Sprintf("%s (%s)", construct.ConstructID, construct.Kind)
		labels[i] = construct.Label
	}
	sections = append(sections, RenderedSection{
		Key:   "A",
		Title: "A — Assessment (what was observed)",
		Rows: []RenderedRow{
			{Label: "Instrument", Value: r.A.Instrument.InstrumentID + "@" + r.A.Instrument.InstrumentVersion},
			{Label: "Item", Value: r.A.Instrument.ItemRef + "@" + r.A.Instrument.ItemVersion},
			{Label: "Prompt", Value: r.A.Instrument.PromptText},
			{Label: "Response", Value: DescribeInterpretation(r.A.Response)},
			{Label: "Constructs", Value: strings.Join(constructs, ", ")},
			{Label: "Observed at", Value: r.A.ObservedAt},
			{Label: "Setting", Value: r.A.Setting},
		},
	})

	if options.RestrictEvidence {
		restricted = append(restricted, "E")
	} else {
		primary, ok := r.PrimaryEvidence()
		unresolved := func(value string) string {
			if !ok {
				return "unresolved"
			}
			return value
		}
		limitations := "none recorded"
		if ok && len(primary.Method.Limitations) > 0 {
			limitations = strings.Join(primary.Method.Limitations, " · ")
		}
		sections = append(sections, RenderedSection{
			Key:   "E",
			Title: "E — Evidence (why it exists, what supports it)",
			Rows: []RenderedRow{
				{Label: "Primary evidence", Value: unresolved(primary.ID)},
				{Label: "Source", Value: unresolved(fmt.Sprintf("%s (%s)", primary.Provenance.SourceRef, primary.Provenance.SourceKind))},
				{Label: "Asserted by", Value: unresolved(primary.Provenance.AssertedBy.Kind + ":" + primary.Provenance.AssertedBy.ID)},
				{Label: "Method", Value: unresolved(fmt.Sprintf("%s@%s — %s", primary.Method.Ref, primary.Method.Version, primary.Method.Description))},
				{Label: "Temporal", Value: unresolved(fmt.Sprintf("observed %s; recorded %s", primary.Temporal.ObservedAt, primary.Temporal.RecordedAt))},
				{Label: "Quality", Value: unresolved(fmt.Sprintf("%s/%s/%s/%s/n=%d", primary.Quality.Directness, primary.Quality.Completeness, primary.Quality.Validity, primary.Quality.Reliability, primary.Quality.SampleSize))},
				{Label: "Method limitations", Value: limitations},
				{Label: "Derived records", Value: fmt.Sprintf("%d", len(r.E.Bundle)-1)},
			},
		})
	}

	if options.RestrictContext {
		restricted = append(restricted, "C")
	} else {
		rows := []RenderedRow{{Label: "Context kind", Value: r.C.Kind}}
		for _, field := range r.C.Fields {
			rows = append(rows, RenderedRow{
				Label:      field.Key,
				Value:      field.Value.Join(", "),
				Provenance: field.AssertedBy.Kind + ":" + field.AssertedBy.ID,
			})
		}
		sections = append(sections, RenderedSection{
			Key:   "C",
			Title: "C — Career context (where this may legitimately be relevant)",
			Rows:  rows,
		})
	}

	if options.RestrictUncertainty {
		restricted = append(restricted, "U")
	} else {
		summary := SummariseUncertainty(r.U)
		rows := []RenderedRow{
			{Label: "Coverage", Value: r.U.Coverage},
			{Label: "Summary", Value: fmt.Sprintf("%d note(s); highest level %s", summary.NoteCount, summary.MaxLevel)},
		}
		for _, note := range r.U.Notes {
			detail := ""
			if note.Mitigation != "" {
				detail = "mitigation: " + note.Mitigation
			}
			rows = append(rows, RenderedRow{
				Label:      note.Code,
				Value:      note.Level + " — " + note.Detail,
				Detail:     detail,
				Provenance: note.NotedBy.Kind + ":" + note.NotedBy.ID,
			})
		}
		sections = append(sections, RenderedSection{
			Key:   "U",
			Title: "U — Uncertainty (cross-cutting over A, E, C)",
			Rows:  rows,
		})
	}

	return RenderedRepresentation{
		Headline:   strings.Join(labels, ", ") + " — single observation",
		Subhead:    fmt.Sprintf("Signal %s · representation %s · %s", r.SignalID, r.ID, r.TransformationRef),
		Sections:   sections,
		Restricted: restricted,
	}
}
