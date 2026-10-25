package domain

import (
	"fmt"
	"strings"
)

// S_0 material: one captured semantic observation.
//
// A signal is what the user actually supplied: an answer, a statement, an
// uploaded artifact, a narrative. It is immutable (I_t). Everything downstream
// derives from it by transformation, never by rewriting it (§11).

const SignalSchemaVersion = "1.0.0"

// Input size limits (§17). Assessment input is data, never executable
// content; the limit is enforced on the canonical form before anything is
// interpreted.
const (
	MaxInputBytes        = 8192
	MaxResponseTextChars = 4000
	MaxContextFields     = 24
)

var SignalSourceKinds = []string{
	"direct-user-response",
	"user-upload",
	"instrument-administration",
	"third-party-report",
	"system-observation",
}

type SignalSource struct {
	Kind string `json:"kind"`
	// Stable reference to where the material came from.
	Ref        string   `json:"ref"`
	Label      string   `json:"label"`
	CapturedBy ActorRef `json:"capturedBy"`
}

type DeclaredField struct {
	Key   string       `json:"key"`
	Value ContextValue `json:"value"`
}

type InputSource struct {
	Kind  string `json:"kind"`
	Ref   string `json:"ref"`
	Label string `json:"label"`
}

// SignalInput is what the user (or another origin) supplied, as intent (I_t).
// This is the only shape the system accepts from outside the domain.
type SignalInput struct {
	InputID      string      `json:"inputId"`
	SubjectID    string      `json:"subjectId"`
	InstrumentID string      `json:"instrumentId"`
	ItemRef      string      `json:"itemRef"`
	Response     RawResponse `json:"response"`
	// Declared context, asserted by the user. Keys are field-guard checked.
	DeclaredContext []DeclaredField `json:"declaredContext"`
	Source          InputSource     `json:"source"`
	SessionRef      string          `json:"sessionRef,omitempty"`
	// Caller-supplied ordering hint; server-recorded time remains authoritative.
	ClientSequence *int `json:"clientSequence,omitempty"`
	// Optional user annotation. It is recorded as an assertion, never as measurement.
	UserNote string `json:"userNote,omitempty"`
}

func ValidateSignalInput(input SignalInput) []Issue {
	c := &checker{}
	InputIDKind.check(c, "inputId", input.InputID)
	SubjectIDKind.check(c, "subjectId", input.SubjectID)
	c.str("instrumentId", input.InstrumentID, 1, 120)
	c.str("itemRef", input.ItemRef, 1, 120)
	checkRawResponse(c, "response", input.Response)
	if input.DeclaredContext == nil {
		c.add("declaredContext", "Invalid input: expected array, received undefined")
	} else {
		c.arr("declaredContext", len(input.DeclaredContext), 1, MaxContextFields)
		for i, field := range input.DeclaredContext {
			c.str(joinPath(idx("declaredContext", i), "key"), field.Key, 1, 64)
			checkContextValue(c, joinPath(idx("declaredContext", i), "value"), field.Value)
		}
	}
	c.enum("source.kind", input.Source.Kind, SignalSourceKinds)
	c.str("source.ref", input.Source.Ref, 1, 200)
	c.str("source.label", input.Source.Label, 1, 160)
	c.optStr("sessionRef", input.SessionRef, 1, 120)
	if input.ClientSequence != nil && *input.ClientSequence < 0 {
		c.add("clientSequence", "Too small: expected number to be >=0")
	}
	c.optStr("userNote", input.UserNote, 1, 1000)
	if input.Response.Kind == "text" && JSLength(input.Response.Text) > MaxResponseTextChars {
		c.add("response.text", "response text exceeds the input size limit")
	}
	return c.result()
}

// Signal is a captured observation (S_0 material). Immutable: never mutated,
// only superseded by a new signal in a new revision (§11).
type Signal struct {
	ID             string                 `json:"id"`
	SchemaVersion  string                 `json:"schemaVersion"`
	SubjectID      string                 `json:"subjectId"`
	Instrument     InstrumentRef          `json:"instrument"`
	Response       ResponseInterpretation `json:"response"`
	Source         SignalSource           `json:"source"`
	CapturedAt     string                 `json:"capturedAt"`
	Context        CareerContext          `json:"context"`
	Evidence       []EvidenceRecord       `json:"evidence"`
	Uncertainty    UncertaintyProfile     `json:"uncertainty"`
	SessionRef     string                 `json:"sessionRef,omitempty"`
	ClientSequence *int                   `json:"clientSequence,omitempty"`
	UserNote       string                 `json:"userNote,omitempty"`
}

func ValidateSignal(s Signal) []Issue {
	c := &checker{}
	SignalIDKind.check(c, "id", s.ID)
	checkSemver(c, "schemaVersion", s.SchemaVersion)
	SubjectIDKind.check(c, "subjectId", s.SubjectID)
	checkInstrumentRef(c, "instrument", s.Instrument)
	checkInterpretation(c, "response", s.Response)
	c.enum("source.kind", s.Source.Kind, SignalSourceKinds)
	c.str("source.ref", s.Source.Ref, 1, 200)
	c.str("source.label", s.Source.Label, 1, 160)
	checkActor(c, "source.capturedBy", s.Source.CapturedBy)
	checkIso(c, "capturedAt", s.CapturedAt)
	checkContext(c, "context", s.Context)
	checkEvidenceBundle(c, "evidence", s.Evidence)
	checkUncertaintyProfile(c, "uncertainty", s.Uncertainty)
	c.optStr("sessionRef", s.SessionRef, 1, 120)
	if s.ClientSequence != nil && *s.ClientSequence < 0 {
		c.add("clientSequence", "Too small: expected number to be >=0")
	}
	c.optStr("userNote", s.UserNote, 1, 1000)
	return c.result()
}

type SignalIdentity struct {
	SignalID     string   `json:"signalId"`
	AssessmentID string   `json:"assessmentId"`
	EvidenceIDs  []string `json:"evidenceIds"`
	ContextID    string   `json:"contextId"`
}

func SignalIdentityOf(s Signal) SignalIdentity {
	ids := make([]string, len(s.Evidence))
	for i, record := range s.Evidence {
		ids[i] = record.ID
	}
	return SignalIdentity{SignalID: s.ID, AssessmentID: s.ID, EvidenceIDs: ids, ContextID: s.Context.ID}
}

// AssertSignalIdentity checks the identity invariant (§6): the assessment,
// evidence, and context records all carry the originating signal identity.
func AssertSignalIdentity(s Signal) []string {
	var issues []string
	for _, record := range s.Evidence {
		if record.SignalID != s.ID {
			issues = append(issues, fmt.Sprintf("evidence %s does not carry the originating signal id %s", record.ID, s.ID))
		}
	}
	if len(s.Evidence) == 0 || s.Evidence[0].ID != s.ID {
		issues = append(issues, "primary evidence id must equal the signal id (SignalId(A_i) = SignalId(E_i))")
	}
	if s.Context.ID != s.ID {
		issues = append(issues, "context id must equal the signal id (SignalId(A_i) = SignalId(C_i))")
	}
	return issues
}

// ----------------------------------------------------------------- capture

type CaptureSignalParams struct {
	// The user's (or origin's) supplied material.
	Input SignalInput
	// Who recorded it.
	CapturedBy ActorRef
	// When the system recorded it.
	RecordedAt string
	// When the observation occurred; defaults to the recorded time.
	ObservedAt    string
	Policy        *UncertaintyPolicy
	SchemaVersion string
}

type SignalWithItem struct {
	Signal Signal
	Item   InstrumentItem
}

func instrumentRefFor(item InstrumentItem) InstrumentRef {
	return InstrumentRef{
		InstrumentID:      item.InstrumentID,
		InstrumentVersion: item.InstrumentVersion,
		ItemRef:           item.ItemRef,
		ItemVersion:       item.ItemVersion,
		PromptText:        item.PromptText,
		ResponseShape:     item.ResponseShape,
		Validated:         item.Validated,
		ValidationRef:     item.ValidationRef,
	}
}

func rejectAll(code string, messages []string, path string) []Rejection {
	out := make([]Rejection, len(messages))
	for i, message := range messages {
		out[i] = NewRejection(code, message, path)
	}
	return out
}

// CaptureSignal is S_0 capture: intent → validated semantic input → immutable
// signal. It is total on valid input and returns structured rejections on
// invalid input. It never repairs input silently and never invents provenance.
func CaptureSignal(params CaptureSignalParams) (SignalWithItem, []Rejection) {
	var rejections []Rejection

	bytes := UTF8ByteLength(Canonicalize(params.Input))
	if bytes > MaxInputBytes {
		rejections = append(rejections, NewRejection("S0.INPUT.SIZE_LIMIT",
			fmt.Sprintf("input is %d bytes, exceeding the %d-byte assessment input limit", bytes, MaxInputBytes), "input"))
	}

	inputIssues := ValidateSignalInput(params.Input)
	rejections = append(rejections, IssuesToRejections("S0.INPUT.SHAPE", inputIssues, "input")...)
	if len(rejections) > 0 {
		return SignalWithItem{}, rejections
	}

	input := params.Input
	item, ok := FindItem(input.ItemRef)
	if !ok {
		return SignalWithItem{}, []Rejection{NewRejection("S0.ITEM.UNKNOWN", fmt.Sprintf("no instrument item is registered for %q", input.ItemRef), "itemRef")}
	}
	if item.InstrumentID != input.InstrumentID {
		return SignalWithItem{}, []Rejection{NewRejection("S0.ITEM.INSTRUMENT_MISMATCH",
			fmt.Sprintf("item %q belongs to instrument %q, not %q", input.ItemRef, item.InstrumentID, input.InstrumentID), "instrumentId")}
	}

	interpretation, problems := InterpretResponse(item, input.Response)
	if len(problems) > 0 {
		return SignalWithItem{}, rejectAll("S0.RESPONSE.INVALID", problems, "response")
	}

	observedAt := params.ObservedAt
	if observedAt == "" {
		observedAt = params.RecordedAt
	}
	policy := DefaultUncertaintyPolicy
	if params.Policy != nil {
		policy = *params.Policy
	}

	var sessionRef, clientSequence any
	if input.SessionRef != "" {
		sessionRef = input.SessionRef
	}
	if input.ClientSequence != nil {
		clientSequence = *input.ClientSequence
	}
	signalID := MintID("sig", map[string]any{
		"subjectId":         input.SubjectID,
		"instrumentId":      item.InstrumentID,
		"instrumentVersion": item.InstrumentVersion,
		"itemRef":           item.ItemRef,
		"itemVersion":       item.ItemVersion,
		"response":          interpretation,
		"context":           input.DeclaredContext,
		"observedAt":        observedAt,
		"sessionRef":        sessionRef,
		"clientSequence":    clientSequence,
	})

	user := UserZeroActor
	fields := make([]ContextFieldInput, len(input.DeclaredContext))
	for i, field := range input.DeclaredContext {
		fields[i] = ContextFieldInput{Key: field.Key, Value: field.Value, AssertedBy: &user}
	}
	context, contextProblems := CreateContext(CreateContextParams{ID: signalID, Kind: "declared", Fields: fields})
	if len(contextProblems) > 0 {
		return SignalWithItem{}, rejectAll("S0.CONTEXT.INVALID", contextProblems, "declaredContext")
	}

	instrumentRef := instrumentRefFor(item)
	quality := QualityFor(QualityParams{
		Instrument:             &instrumentRef,
		ResponseComplete:       true,
		ObservedAt:             observedAt,
		Now:                    params.RecordedAt,
		HasIndependentEvidence: false,
		Notes:                  "Single self-report observation; no corroborating evidence is referenced yet.",
	})

	methodKind := "self-report-item"
	if item.ResponseShape == "open-text" {
		methodKind = "free-text-response"
	}
	var sequence *int
	if input.ClientSequence != nil {
		sequence = intPtr(*input.ClientSequence)
	}
	primary, evidenceProblems := CreatePrimaryEvidence(PrimaryEvidenceParams{
		SignalID:          signalID,
		SourceRef:         input.Source.Ref,
		SourceKind:        input.Source.Kind,
		AssertedBy:        &user,
		CapturedBy:        params.CapturedBy,
		MethodRef:         "method." + item.ResponseShape + ".v1",
		MethodKind:        methodKind,
		MethodVersion:     "1.0.0",
		MethodDescription: fmt.Sprintf("Administration of %s (%s@%s) with a %s response shape.", item.ItemRef, item.InstrumentID, item.InstrumentVersion, item.ResponseShape),
		Instrument:        &instrumentRef,
		Limitations:       item.Limitations,
		Temporal: TemporalRecord{
			ObservedAt: observedAt,
			RecordedAt: params.RecordedAt,
			SessionRef: input.SessionRef,
			Sequence:   sequence,
		},
		Quality: quality,
	})
	if len(evidenceProblems) > 0 {
		return SignalWithItem{}, rejectAll("S0.EVIDENCE.INVALID", evidenceProblems, "evidence")
	}

	bundle, bundleProblems := CreateEvidenceBundle([]EvidenceRecord{primary})
	if len(bundleProblems) > 0 {
		return SignalWithItem{}, rejectAll("S0.EVIDENCE.BUNDLE", bundleProblems, "evidence")
	}

	uncertaintyInput := AssessUncertaintyInput{
		SignalID:              signalID,
		HasResponse:           true,
		ResponseKind:          interpretation.Kind,
		InstrumentID:          item.InstrumentID,
		MethodRef:             "method." + item.ResponseShape + ".v1",
		ObservedAtIso:         observedAt,
		NowIso:                params.RecordedAt,
		ContextFieldCount:     len(context.Fields),
		QualityKnown:          quality.Completeness != "unknown",
		HasSupportingEvidence: false,
	}
	switch interpretation.Kind {
	case "text":
		uncertaintyInput.ResponseText = interpretation.Text
	case "categorical":
		uncertaintyInput.ResponseText = strings.Join(interpretation.SelectedLabels, " ")
	}
	capturedBy := params.CapturedBy
	uncertainty := AssessUncertainty(uncertaintyInput, AssessUncertaintyOptions{Policy: &policy, NotedBy: &capturedBy})

	schemaVersion := params.SchemaVersion
	if schemaVersion == "" {
		schemaVersion = SignalSchemaVersion
	}
	var clientSeq *int
	if input.ClientSequence != nil {
		clientSeq = intPtr(*input.ClientSequence)
	}
	candidate := Signal{
		ID:            signalID,
		SchemaVersion: schemaVersion,
		SubjectID:     input.SubjectID,
		Instrument:    instrumentRef,
		Response:      interpretation,
		Source: SignalSource{
			Kind:       input.Source.Kind,
			Ref:        input.Source.Ref,
			Label:      input.Source.Label,
			CapturedBy: params.CapturedBy,
		},
		CapturedAt:     params.RecordedAt,
		Context:        context,
		Evidence:       bundle,
		Uncertainty:    uncertainty,
		SessionRef:     input.SessionRef,
		ClientSequence: clientSeq,
		UserNote:       input.UserNote,
	}

	if issues := ValidateSignal(candidate); len(issues) > 0 {
		return SignalWithItem{}, IssuesToRejections("S0.SIGNAL.SHAPE", issues, "signal")
	}
	if identityIssues := AssertSignalIdentity(candidate); len(identityIssues) > 0 {
		return SignalWithItem{}, rejectAll("S0.SIGNAL.IDENTITY", identityIssues, "signal")
	}
	return SignalWithItem{Signal: candidate, Item: item}, nil
}
