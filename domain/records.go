package domain

import (
	"fmt"
	"sort"
)

// The durable, auditable record types.
//
// Records are the historical layer (§11). They are appended, versioned, and
// digests are chained; nothing here is ever edited in place. Records also
// carry the 0 boundary: any record representing a decision must name a user
// actor, and any record representing model output is marked as a proposal.

const RecordsSchemaVersion = "1.0.0"

type AssessmentRecord struct {
	ID            string `json:"id"`
	SchemaVersion string `json:"schemaVersion"`
	RecordType    string `json:"recordType"`
	SubjectID     string `json:"subjectId"`
	// Monotonic revision; history is append-only.
	Revision          int      `json:"revision"`
	PreviousRecordRef string   `json:"previousRecordRef,omitempty"`
	SessionRef        string   `json:"sessionRef"`
	StartedAt         string   `json:"startedAt"`
	RecordedAt        string   `json:"recordedAt"`
	RecordedBy        ActorRef `json:"recordedBy"`
	// Signals captured during the sitting, in capture order.
	SignalRefs []string `json:"signalRefs"`
	// Representations derived from those signals at record time.
	RepresentationRefs []string `json:"representationRefs"`
	InstrumentRefs     []string `json:"instrumentRefs"`
	ContentDigest      Digest   `json:"contentDigest"`
	// Explicit statement of what this record does not contain.
	Exclusions []string `json:"exclusions"`
}

func ValidateAssessmentRecord(r AssessmentRecord) []Issue {
	c := &checker{}
	RecordIDKind.check(c, "id", r.ID)
	checkSemver(c, "schemaVersion", r.SchemaVersion)
	if r.RecordType != "assessment-record" {
		c.add("recordType", `Invalid input: expected "assessment-record"`)
	}
	SubjectIDKind.check(c, "subjectId", r.SubjectID)
	c.intRange("revision", r.Revision, 1, 0, false)
	c.optStr("previousRecordRef", r.PreviousRecordRef, 1, -1)
	c.str("sessionRef", r.SessionRef, 1, -1)
	checkIso(c, "startedAt", r.StartedAt)
	checkIso(c, "recordedAt", r.RecordedAt)
	checkActor(c, "recordedBy", r.RecordedBy)
	c.strings("signalRefs", r.SignalRefs, 1, -1, 1, -1)
	c.strings("representationRefs", r.RepresentationRefs, 1, -1, 0, -1)
	c.strings("instrumentRefs", r.InstrumentRefs, 1, -1, 1, -1)
	checkDigest(c, "contentDigest", r.ContentDigest)
	c.strings("exclusions", r.Exclusions, 1, 400, 0, 8)
	return c.result()
}

type SessionRecord struct {
	ID           string   `json:"id"`
	SubjectID    string   `json:"subjectId"`
	StartedAt    string   `json:"startedAt"`
	EndedAt      string   `json:"endedAt,omitempty"`
	RecordedBy   ActorRef `json:"recordedBy"`
	PromptSetRef string   `json:"promptSetRef,omitempty"`
	SignalRefs   []string `json:"signalRefs"`
}

func ValidateSessionRecord(r SessionRecord) []Issue {
	c := &checker{}
	SessionIDKind.check(c, "id", r.ID)
	SubjectIDKind.check(c, "subjectId", r.SubjectID)
	checkIso(c, "startedAt", r.StartedAt)
	checkOptIso(c, "endedAt", r.EndedAt)
	checkActor(c, "recordedBy", r.RecordedBy)
	c.optStr("promptSetRef", r.PromptSetRef, 1, 160)
	c.strings("signalRefs", r.SignalRefs, 1, -1, 0, -1)
	return c.result()
}

var DecisionKinds = []string{
	"accept-representation",
	"reject-representation",
	"correct-representation",
	"explore-pathway",
	"dismiss-pathway",
	"pursue-pathway",
	"defer-pathway",
	"no-decision-yet",
}

// UserDecisionRecord is a decision made by user 0. The system records
// decisions; it never makes them.
type UserDecisionRecord struct {
	ID            string   `json:"id"`
	SchemaVersion string   `json:"schemaVersion"`
	RecordType    string   `json:"recordType"`
	SubjectID     string   `json:"subjectId"`
	DecisionKind  string   `json:"decisionKind"`
	TargetRef     string   `json:"targetRef"`
	Rationale     string   `json:"rationale,omitempty"`
	DecidedAt     string   `json:"decidedAt"`
	DecidedBy     ActorRef `json:"decidedBy"`
	// What the system showed the user when they decided; keeps the UI honest.
	PresentationRef string `json:"presentationRef,omitempty"`
}

func ValidateUserDecisionRecord(r UserDecisionRecord) []Issue {
	c := &checker{}
	RecordIDKind.check(c, "id", r.ID)
	checkSemver(c, "schemaVersion", r.SchemaVersion)
	if r.RecordType != "user-decision" {
		c.add("recordType", `Invalid input: expected "user-decision"`)
	}
	SubjectIDKind.check(c, "subjectId", r.SubjectID)
	c.enum("decisionKind", r.DecisionKind, DecisionKinds)
	c.str("targetRef", r.TargetRef, 1, -1)
	c.optStr("rationale", r.Rationale, 0, 1000)
	checkIso(c, "decidedAt", r.DecidedAt)
	checkActor(c, "decidedBy", r.DecidedBy)
	c.optStr("presentationRef", r.PresentationRef, 1, 200)
	if r.DecidedBy.Kind != ActorUser {
		c.add("decidedBy", "a decision record must be attributed to the user (0); the system records decisions, it does not make them")
	}
	return c.result()
}

// CorrectionRecord is a user correction to a representation. Corrections do
// not edit history: they add a correction record and a superseding
// representation.
type CorrectionRecord struct {
	ID                    string   `json:"id"`
	SchemaVersion         string   `json:"schemaVersion"`
	RecordType            string   `json:"recordType"`
	SubjectID             string   `json:"subjectId"`
	TargetRef             string   `json:"targetRef"`
	Dimension             string   `json:"dimension"`
	Field                 string   `json:"field"`
	PreviousValueSummary  string   `json:"previousValueSummary"`
	CorrectedValueSummary string   `json:"correctedValueSummary"`
	Reason                string   `json:"reason"`
	RecordedAt            string   `json:"recordedAt"`
	RecordedBy            ActorRef `json:"recordedBy"`
	// The representation produced to supersede the corrected one.
	SupersedingRepresentationRef string `json:"supersedingRepresentationRef,omitempty"`
}

func ValidateCorrectionRecord(r CorrectionRecord) []Issue {
	c := &checker{}
	RecordIDKind.check(c, "id", r.ID)
	checkSemver(c, "schemaVersion", r.SchemaVersion)
	if r.RecordType != "correction" {
		c.add("recordType", `Invalid input: expected "correction"`)
	}
	SubjectIDKind.check(c, "subjectId", r.SubjectID)
	c.str("targetRef", r.TargetRef, 1, -1)
	c.enum("dimension", r.Dimension, UncertaintyDimensions)
	c.str("field", r.Field, 1, 120)
	c.str("previousValueSummary", r.PreviousValueSummary, 1, 400)
	c.str("correctedValueSummary", r.CorrectedValueSummary, 1, 400)
	c.str("reason", r.Reason, 1, 1000)
	checkIso(c, "recordedAt", r.RecordedAt)
	checkActor(c, "recordedBy", r.RecordedBy)
	c.optStr("supersedingRepresentationRef", r.SupersedingRepresentationRef, 1, -1)
	if r.RecordedBy.Kind != ActorUser {
		c.add("recordedBy", "corrections to a representation are made by the user (0)")
	}
	return c.result()
}

var ProposalKinds = []string{"candidate-question", "semantic-extraction", "pathway-explanation", "summary", "uncertainty-review"}
var ProposalStatuses = []string{"proposed", "accepted-by-user", "rejected-by-user", "expired"}

// ProposalRecord is model-assisted content. §5: a model proposes; it does not
// become the source of truth. Proposals are recorded with full model metadata,
// cannot carry evidence references of their own, and require an explicit user
// decision to be used.
type ProposalRecord struct {
	ID            string `json:"id"`
	SchemaVersion string `json:"schemaVersion"`
	RecordType    string `json:"recordType"`
	SubjectID     string `json:"subjectId"`
	ProposalKind  string `json:"proposalKind"`
	// Model metadata is mandatory (§5).
	ModelMetadata ProviderMetadata `json:"modelMetadata"`
	Summary       string           `json:"summary"`
	// Proposals never carry evidence of their own.
	EvidenceRefs []string `json:"evidenceRefs"`
	// Model output is uncertain by construction, and the uncertainty travels
	// with the proposal rather than being attached later (§3, §5).
	Uncertainty []UncertaintyNote `json:"uncertainty"`
	TargetRefs  []string          `json:"targetRefs"`
	Status      string            `json:"status"`
	DecidedBy   *ActorRef         `json:"decidedBy,omitempty"`
	DecidedAt   string            `json:"decidedAt,omitempty"`
	RecordedAt  string            `json:"recordedAt"`
	RecordedBy  ActorRef          `json:"recordedBy"`
}

func ValidateProposalRecord(r ProposalRecord) []Issue {
	c := &checker{}
	ProposalIDKind.check(c, "id", r.ID)
	checkSemver(c, "schemaVersion", r.SchemaVersion)
	if r.RecordType != "proposal" {
		c.add("recordType", `Invalid input: expected "proposal"`)
	}
	SubjectIDKind.check(c, "subjectId", r.SubjectID)
	c.enum("proposalKind", r.ProposalKind, ProposalKinds)
	checkProviderMetadata(c, "modelMetadata", r.ModelMetadata)
	c.str("summary", r.Summary, 1, 4000)
	c.strings("evidenceRefs", r.EvidenceRefs, 1, -1, 0, 0)
	c.arr("uncertainty", len(r.Uncertainty), 1, 4)
	for i, note := range r.Uncertainty {
		checkUncertaintyNote(c, idx("uncertainty", i), note)
	}
	c.strings("targetRefs", r.TargetRefs, 1, -1, 0, 16)
	c.enum("status", r.Status, ProposalStatuses)
	if r.DecidedBy != nil {
		checkActor(c, "decidedBy", *r.DecidedBy)
	}
	checkOptIso(c, "decidedAt", r.DecidedAt)
	checkIso(c, "recordedAt", r.RecordedAt)
	checkActor(c, "recordedBy", r.RecordedBy)
	if r.Status != "proposed" && (r.DecidedBy == nil || r.DecidedBy.Kind != ActorUser) {
		c.add("decidedBy", `a proposal status other than "proposed" must be decided by the user (0)`)
	}
	return c.result()
}

// ProposalAsEvidenceRejection is used when something tries to treat a
// proposal as evidence.
func ProposalAsEvidenceRejection(proposalID string) Rejection {
	return NewRejection("INV.EVIDENCE.PROMOTED-CLAIM",
		fmt.Sprintf("proposal %s cannot be used as evidence: model output supports a claim only after the user accepts it and independent evidence exists", proposalID),
		"evidenceRefs")
}

// ---------------------------------------------------------------- factory

var recordExclusions = []string{
	"No scalar career score is stored on this record.",
	"No career decision is stored on this record: decisions belong to user 0 and are recorded separately.",
}

type CreateAssessmentRecordParams struct {
	SubjectID          string
	SessionRef         string
	StartedAt          string
	RecordedAt         string
	RecordedBy         ActorRef
	SignalRefs         []string
	RepresentationRefs []string
	InstrumentRefs     []string
	PreviousRecordRef  string
	Revision           int
	Hasher             Hasher
}

func CreateAssessmentRecord(params CreateAssessmentRecordParams) (AssessmentRecord, []Rejection) {
	if len(params.SignalRefs) == 0 {
		return AssessmentRecord{}, []Rejection{NewRejection("S4.RECORD.NO-SIGNALS", "an assessment record must reference the signals it contains", "signalRefs")}
	}
	hasher := params.Hasher
	if hasher == nil {
		hasher = FNV1a64Hasher
	}
	revision := params.Revision
	if revision == 0 {
		revision = 1
	}
	id := MintID("rec", map[string]any{
		"kind":       "assessment-record",
		"subjectId":  params.SubjectID,
		"sessionRef": params.SessionRef,
		"revision":   revision,
		"recordedAt": params.RecordedAt,
	})
	content := map[string]any{
		"subjectId":          params.SubjectID,
		"sessionRef":         params.SessionRef,
		"signalRefs":         CopyStrings(params.SignalRefs),
		"representationRefs": CopyStrings(params.RepresentationRefs),
		"instrumentRefs":     CopyStrings(params.InstrumentRefs),
	}
	candidate := AssessmentRecord{
		ID:                 id,
		SchemaVersion:      RecordsSchemaVersion,
		RecordType:         "assessment-record",
		SubjectID:          params.SubjectID,
		Revision:           revision,
		PreviousRecordRef:  params.PreviousRecordRef,
		SessionRef:         params.SessionRef,
		StartedAt:          params.StartedAt,
		RecordedAt:         params.RecordedAt,
		RecordedBy:         params.RecordedBy,
		SignalRefs:         CopyStrings(params.SignalRefs),
		RepresentationRefs: CopyStrings(params.RepresentationRefs),
		InstrumentRefs:     CopyStrings(params.InstrumentRefs),
		ContentDigest:      DigestOf(hasher, content, "assessment-record:"+id),
		Exclusions:         CopyStrings(recordExclusions),
	}
	if issues := ValidateAssessmentRecord(candidate); len(issues) > 0 {
		return AssessmentRecord{}, IssuesToRejections("S4.RECORD.SHAPE", issues, "record")
	}
	return candidate, nil
}

type CreateSessionRecordParams struct {
	SubjectID    string
	StartedAt    string
	RecordedBy   ActorRef
	SignalRefs   []string
	PromptSetRef string
	EndedAt      string
}

func CreateSessionRecord(params CreateSessionRecordParams) (SessionRecord, []Rejection) {
	record := SessionRecord{
		ID:           MintID("ses", map[string]any{"subjectId": params.SubjectID, "startedAt": params.StartedAt}),
		SubjectID:    params.SubjectID,
		StartedAt:    params.StartedAt,
		EndedAt:      params.EndedAt,
		RecordedBy:   params.RecordedBy,
		PromptSetRef: params.PromptSetRef,
		SignalRefs:   CopyStrings(params.SignalRefs),
	}
	if issues := ValidateSessionRecord(record); len(issues) > 0 {
		return SessionRecord{}, IssuesToRejections("S4.SESSION.SHAPE", issues, "session")
	}
	return record, nil
}

type CreateUserDecisionParams struct {
	SubjectID       string
	DecisionKind    string
	TargetRef       string
	DecidedAt       string
	DecidedBy       ActorRef
	Rationale       string
	PresentationRef string
}

func CreateUserDecisionRecord(params CreateUserDecisionParams) (UserDecisionRecord, []Rejection) {
	if params.DecidedBy.Kind != ActorUser {
		return UserDecisionRecord{}, []Rejection{NewRejection("INV.PATHWAY.USER-AUTHORITY",
			fmt.Sprintf("decision records are attributable to user 0 only; received %q", params.DecidedBy.Kind), "decidedBy")}
	}
	record := UserDecisionRecord{
		ID: MintID("rec", map[string]any{
			"kind":         "user-decision",
			"subjectId":    params.SubjectID,
			"decisionKind": params.DecisionKind,
			"targetRef":    params.TargetRef,
			"decidedAt":    params.DecidedAt,
		}),
		SchemaVersion:   RecordsSchemaVersion,
		RecordType:      "user-decision",
		SubjectID:       params.SubjectID,
		DecisionKind:    params.DecisionKind,
		TargetRef:       params.TargetRef,
		Rationale:       params.Rationale,
		DecidedAt:       params.DecidedAt,
		DecidedBy:       params.DecidedBy,
		PresentationRef: params.PresentationRef,
	}
	if issues := ValidateUserDecisionRecord(record); len(issues) > 0 {
		return UserDecisionRecord{}, IssuesToRejections("S4.DECISION.SHAPE", issues, "decision")
	}
	return record, nil
}

type CreateProposalParams struct {
	SubjectID     string
	ProposalKind  string
	ModelMetadata ProviderMetadata
	Summary       string
	TargetRefs    []string
	RecordedAt    string
	RecordedBy    ActorRef
}

func CreateProposalRecord(params CreateProposalParams) (ProposalRecord, []Rejection) {
	m := params.ModelMetadata
	id := MintID("prp", map[string]any{
		"subjectId": params.SubjectID,
		"kind":      params.ProposalKind,
		"model":     fmt.Sprintf("%s/%s@%s", m.Provider, m.Model, m.ModelVersion),
		"outputRef": m.OutputRef,
	})
	d := mustDescriptor("U-MODEL-GENERATED")
	noteID := MintID("unc", map[string]any{
		"kind":         "model-generated",
		"outputRef":    m.OutputRef,
		"proposalKind": params.ProposalKind,
	})
	uncertainty := []UncertaintyNote{{
		ID:         noteID,
		Code:       "U-MODEL-GENERATED",
		Source:     d.Source,
		Kind:       d.Kind,
		Level:      d.DefaultLevel,
		Detail:     fmt.Sprintf("Produced by %s/%s@%s under instruction %s; it is a proposal, not an observation.", m.Provider, m.Model, m.ModelVersion, m.InstructionVersion),
		Affects:    CopyStrings(d.Affects),
		Mitigation: d.Mitigation,
		NotedBy:    params.RecordedBy,
	}}
	record := ProposalRecord{
		ID:            id,
		SchemaVersion: RecordsSchemaVersion,
		RecordType:    "proposal",
		SubjectID:     params.SubjectID,
		ProposalKind:  params.ProposalKind,
		ModelMetadata: m,
		Summary:       params.Summary,
		EvidenceRefs:  []string{},
		Uncertainty:   uncertainty,
		TargetRefs:    CopyStrings(params.TargetRefs),
		Status:        "proposed",
		RecordedAt:    params.RecordedAt,
		RecordedBy:    params.RecordedBy,
	}
	if issues := ValidateProposalRecord(record); len(issues) > 0 {
		return ProposalRecord{}, IssuesToRejections("S3.PROPOSAL.SHAPE", issues, "proposal")
	}
	return record, nil
}

type CreateCorrectionParams struct {
	SubjectID                    string
	TargetRef                    string
	Dimension                    string
	Field                        string
	PreviousValueSummary         string
	CorrectedValueSummary        string
	Reason                       string
	RecordedAt                   string
	RecordedBy                   ActorRef
	SupersedingRepresentationRef string
}

func CreateCorrectionRecord(params CreateCorrectionParams) (CorrectionRecord, []Rejection) {
	if params.RecordedBy.Kind != ActorUser {
		return CorrectionRecord{}, []Rejection{NewRejection("INV.VERSION.NON-DESTRUCTIVE", "only the user (0) may correct their own representation", "recordedBy")}
	}
	record := CorrectionRecord{
		ID: MintID("rec", map[string]any{
			"kind":       "correction",
			"subjectId":  params.SubjectID,
			"targetRef":  params.TargetRef,
			"dimension":  params.Dimension,
			"field":      params.Field,
			"recordedAt": params.RecordedAt,
		}),
		SchemaVersion:                RecordsSchemaVersion,
		RecordType:                   "correction",
		SubjectID:                    params.SubjectID,
		TargetRef:                    params.TargetRef,
		Dimension:                    params.Dimension,
		Field:                        params.Field,
		PreviousValueSummary:         params.PreviousValueSummary,
		CorrectedValueSummary:        params.CorrectedValueSummary,
		Reason:                       params.Reason,
		RecordedAt:                   params.RecordedAt,
		RecordedBy:                   params.RecordedBy,
		SupersedingRepresentationRef: params.SupersedingRepresentationRef,
	}
	if issues := ValidateCorrectionRecord(record); len(issues) > 0 {
		return CorrectionRecord{}, IssuesToRejections("S4.CORRECTION.SHAPE", issues, "correction")
	}
	return record, nil
}

// ------------------------------------------------------------- versioning
//
// Append-only versioned records. §11: assessment observations are immutable
// or versioned; historical state is never silently overwritten. Persistence is
// replaceable (§16), but the semantics of history (append-only, chained
// digests, no overwrite) are not.

type Revision struct {
	RevisionID string   `json:"revisionId"`
	Version    int      `json:"version"`
	RecordedAt string   `json:"recordedAt"`
	RecordedBy ActorRef `json:"recordedBy"`
	Note       string   `json:"note,omitempty"`
	// Digest of this revision's payload.
	PayloadDigest Digest `json:"payloadDigest"`
	// Digest of the previous revision, forming a chain of custody.
	PreviousDigest *Digest `json:"previousDigest,omitempty"`
	Supersedes     string  `json:"supersedes,omitempty"`
}

// VersionedRecord is immutable by construction: every operation returns a new
// record value and never modifies the slices or map of an existing one.
type VersionedRecord[T any] struct {
	RecordID       string       `json:"recordId"`
	RecordType     string       `json:"recordType"`
	SubjectID      string       `json:"subjectId"`
	CreatedAt      string       `json:"createdAt"`
	CreatedBy      ActorRef     `json:"createdBy"`
	HeadRevisionID string       `json:"headRevisionId"`
	Revisions      []Revision   `json:"revisions"`
	Payloads       map[string]T `json:"payloads"`
}

type CreateVersionedRecordParams[T any] struct {
	RecordID   string
	RecordType string
	SubjectID  string
	Payload    T
	RecordedAt string
	RecordedBy ActorRef
	Note       string
	Hasher     Hasher
}

func CreateVersionedRecord[T any](params CreateVersionedRecordParams[T]) VersionedRecord[T] {
	hasher := params.Hasher
	if hasher == nil {
		hasher = FNV1a64Hasher
	}
	revisionID := MintID("rev", map[string]any{"recordId": params.RecordID, "version": 1, "recordedAt": params.RecordedAt})
	revision := Revision{
		RevisionID:    revisionID,
		Version:       1,
		RecordedAt:    params.RecordedAt,
		RecordedBy:    params.RecordedBy,
		Note:          params.Note,
		PayloadDigest: DigestOf(hasher, params.Payload, "revision:"+revisionID),
	}
	return VersionedRecord[T]{
		RecordID:       params.RecordID,
		RecordType:     params.RecordType,
		SubjectID:      params.SubjectID,
		CreatedAt:      params.RecordedAt,
		CreatedBy:      params.RecordedBy,
		HeadRevisionID: revisionID,
		Revisions:      []Revision{revision},
		Payloads:       map[string]T{revisionID: params.Payload},
	}
}

type AppendRevisionParams[T any] struct {
	Record     VersionedRecord[T]
	Payload    T
	RecordedAt string
	RecordedBy ActorRef
	Note       string
	Hasher     Hasher
}

// AppendRevision returns a new record; the previous record value remains
// valid and unchanged, which is what makes history non-destructive.
func AppendRevision[T any](params AppendRevisionParams[T]) (VersionedRecord[T], []Rejection) {
	hasher := params.Hasher
	if hasher == nil {
		hasher = FNV1a64Hasher
	}
	var head *Revision
	for i := range params.Record.Revisions {
		if params.Record.Revisions[i].RevisionID == params.Record.HeadRevisionID {
			head = &params.Record.Revisions[i]
		}
	}
	if head == nil {
		return VersionedRecord[T]{}, []Rejection{NewRejection("INV.VERSION.NON-DESTRUCTIVE", "record head revision is missing from history", "revisions")}
	}
	version := head.Version + 1
	revisionID := MintID("rev", map[string]any{"recordId": params.Record.RecordID, "version": version, "recordedAt": params.RecordedAt})
	previous := head.PayloadDigest
	revision := Revision{
		RevisionID:     revisionID,
		Version:        version,
		RecordedAt:     params.RecordedAt,
		RecordedBy:     params.RecordedBy,
		Note:           params.Note,
		PayloadDigest:  DigestOf(hasher, params.Payload, "revision:"+revisionID),
		PreviousDigest: &previous,
		Supersedes:     head.RevisionID,
	}
	next := params.Record
	next.HeadRevisionID = revisionID
	next.Revisions = append(append([]Revision{}, params.Record.Revisions...), revision)
	next.Payloads = make(map[string]T, len(params.Record.Payloads)+1)
	for key, value := range params.Record.Payloads {
		next.Payloads[key] = value
	}
	next.Payloads[revisionID] = params.Payload
	return next, nil
}

// AttemptOverwrite makes the refusal to mutate a stored revision explicit and
// testable rather than implied.
func AttemptOverwrite[T any](record VersionedRecord[T], revisionID string) []Rejection {
	return []Rejection{NewRejection("INV.VERSION.NON-DESTRUCTIVE",
		fmt.Sprintf("refusing to overwrite revision %s of record %s: historical representation is immutable", revisionID, record.RecordID),
		revisionID)}
}

// AttemptDuplicateHead proves the store refuses duplicate history.
func AttemptDuplicateHead[T any](record VersionedRecord[T]) []Rejection {
	return []Rejection{NewRejection("INV.VERSION.NON-DESTRUCTIVE",
		fmt.Sprintf("record %s already has revision %s; appending it again would duplicate history", record.RecordID, record.HeadRevisionID),
		record.HeadRevisionID)}
}

func HeadPayload[T any](record VersionedRecord[T]) (T, bool) {
	payload, ok := record.Payloads[record.HeadRevisionID]
	return payload, ok
}

func AsOfVersion[T any](record VersionedRecord[T], version int) (T, bool) {
	for _, revision := range record.Revisions {
		if revision.Version == version {
			payload, ok := record.Payloads[revision.RevisionID]
			return payload, ok
		}
	}
	var zero T
	return zero, false
}

type ChainVerification struct {
	Valid            bool        `json:"valid"`
	CheckedRevisions int         `json:"checkedRevisions"`
	Rejections       []Rejection `json:"rejections"`
}

// VerifyChain verifies the digest chain of a record; tampering with history
// is detectable.
func VerifyChain[T any](record VersionedRecord[T], hasher Hasher) ChainVerification {
	if hasher == nil {
		hasher = FNV1a64Hasher
	}
	rejections := []Rejection{}
	ordered := append([]Revision{}, record.Revisions...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Version < ordered[j].Version })
	var previous *Revision
	for i := range ordered {
		revision := ordered[i]
		payload, ok := record.Payloads[revision.RevisionID]
		if !ok {
			rejections = append(rejections, NewRejection("INV.VERSION.NON-DESTRUCTIVE", fmt.Sprintf("revision %s has no payload", revision.RevisionID), "payloads"))
			continue
		}
		digest := DigestOf(hasher, payload, "revision:"+revision.RevisionID)
		if digest.Value != revision.PayloadDigest.Value {
			rejections = append(rejections, NewRejection("INV.VERSION.NON-DESTRUCTIVE",
				fmt.Sprintf("revision %s payload digest does not match its recorded digest", revision.RevisionID), "payloads"))
		}
		if previous != nil && (revision.PreviousDigest == nil || revision.PreviousDigest.Value != previous.PayloadDigest.Value) {
			rejections = append(rejections, NewRejection("INV.VERSION.NON-DESTRUCTIVE",
				fmt.Sprintf("revision %s does not chain to its predecessor", revision.RevisionID), "revisions"))
		}
		previous = &ordered[i]
	}
	return ChainVerification{Valid: len(rejections) == 0, CheckedRevisions: len(ordered), Rejections: rejections}
}
