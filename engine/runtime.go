package engine

import (
	"fmt"
	"sync"

	"career107/domain"
)

// The Career Assessment 107 runtime.
//
// The runtime is infrastructure: it captures S_0 input, invokes capabilities
// through the authorization boundary, records transformations and executions,
// and exposes the resulting domain state. It contains no career semantics of
// its own, and it cannot decide anything on the user's behalf.
//
//	I_t → S0 → S1 → S2 → S3 → S4 → O_t → I_{t+1}

const (
	RuntimeVersion     = "0.1.0"
	DefaultGrantBudget = 64
)

type Options struct {
	Clock             domain.Clock
	Hasher            domain.Hasher
	RuntimeID         string
	Cloud107          Cloud107Adapter
	Model             ModelProvider
	Persistence       CareerPersistence
	Authorization     *AuthorizationService
	UncertaintyPolicy *domain.UncertaintyPolicy
	GrantBudget       int
	// IssueDefaultGrant issues the development grant. When nil it defaults to
	// true for a freshly constructed runtime, and to false when the caller
	// supplies its own AuthorizationService — a caller who brings
	// authorization sets the policy.
	IssueDefaultGrant *bool
}

type DecomposeOutcome struct {
	Representation domain.Representation1to3
	Transformation domain.TransformationRecord
	Execution      ExecutionRecord
	Trace          PipelineTrace
}

type PathwayOutcome struct {
	PathwaySet     domain.PathwaySet
	Transformation domain.TransformationRecord
	Execution      ExecutionRecord
	Trace          PipelineTrace
}

type ProposalOutcome struct {
	Proposal       domain.ProposalRecord
	Transformation domain.TransformationRecord
	Execution      ExecutionRecord
	Trace          PipelineTrace
	Findings       []Finding
}

type CaptureOutcome struct {
	Signal   domain.Signal
	Previous *domain.Signal
}

// CareerRuntime is safe for concurrent use: every operation holds the runtime
// lock, so operations are applied one at a time, in order.
type CareerRuntime struct {
	RuntimeID         string
	Registry          *CapabilityRegistry
	Authorization     *AuthorizationService
	Artifacts         *ArtifactStore
	Traces            *TraceLog
	Cloud107          Cloud107Adapter
	Persistence       CareerPersistence
	UncertaintyPolicy domain.UncertaintyPolicy

	mu     sync.Mutex
	clock  domain.Clock
	hasher domain.Hasher
	model  ModelProvider

	signalOrder     []string
	signals         map[string]domain.Signal
	reprOrder       []string
	representations map[string]domain.Representation1to3
	setOrder        []string
	pathwaySets     map[string]domain.PathwaySet

	transformations   []domain.TransformationRecord
	executions        []ExecutionRecord
	decisions         []domain.UserDecisionRecord
	corrections       []domain.CorrectionRecord
	proposals         []domain.ProposalRecord
	assessmentRecords []domain.AssessmentRecord
	sessions          map[string]domain.SessionRecord
}

func New(options Options) *CareerRuntime {
	r := &CareerRuntime{
		RuntimeID:       options.RuntimeID,
		Registry:        NewCapabilityRegistry(),
		Traces:          &TraceLog{},
		clock:           options.Clock,
		hasher:          options.Hasher,
		model:           options.Model,
		signals:         map[string]domain.Signal{},
		representations: map[string]domain.Representation1to3{},
		pathwaySets:     map[string]domain.PathwaySet{},
		sessions:        map[string]domain.SessionRecord{},
	}
	if r.clock == nil {
		r.clock = domain.SystemClock
	}
	if r.hasher == nil {
		r.hasher = domain.FNV1a64Hasher
	}
	if r.RuntimeID == "" {
		r.RuntimeID = "career107-runtime"
	}
	r.UncertaintyPolicy = domain.DefaultUncertaintyPolicy
	if options.UncertaintyPolicy != nil {
		r.UncertaintyPolicy = *options.UncertaintyPolicy
	}
	r.Persistence = options.Persistence
	if r.Persistence == nil {
		r.Persistence = NewInMemoryCareerPersistence()
	}
	r.Authorization = options.Authorization
	if r.Authorization == nil {
		r.Authorization = NewAuthorizationService()
	}
	r.Artifacts = NewArtifactStore(r.hasher)
	r.Cloud107 = options.Cloud107
	if r.Cloud107 == nil {
		r.Cloud107 = &InProcessCloud107Adapter{RuntimeID: r.RuntimeID}
	}

	for _, handler := range CreateDomainHandlers(HandlerDeps{Clock: r.clock, Model: options.Model}) {
		if err := r.Registry.Register(handler); err != nil {
			panic(err)
		}
	}

	issueDefault := options.Authorization == nil
	if options.IssueDefaultGrant != nil {
		issueDefault = *options.IssueDefaultGrant
	}
	if issueDefault {
		budget := options.GrantBudget
		if budget == 0 {
			budget = DefaultGrantBudget
		}
		r.Authorization.IssueGrant(IssueGrantParams{
			Actor:      domain.UserZeroActor,
			Scopes:     CapabilityScopesAll(),
			SubjectIDs: []string{"*"},
			IssuedAt:   r.clock.Now(),
			Budget: map[string]int{
				domain.CapabilityDecompose1to3:    budget,
				domain.CapabilityMapPathways:      budget,
				domain.CapabilityProposeQuestions: budget,
			},
		})
	}
	return r
}

func (r *CareerRuntime) persist(document PersistedDocument) {
	_ = r.Persistence.Put(document)
}

// ---------------------------------------------------------------- S0 capture

type CaptureOptions struct {
	CapturedBy *domain.ActorRef
	ObservedAt string
}

func (r *CareerRuntime) CaptureSignal(input domain.SignalInput, options CaptureOptions) (CaptureOutcome, []domain.Rejection) {
	r.mu.Lock()
	defer r.mu.Unlock()

	recordedAt := r.clock.Now()
	const operation = "career.assessment.capture-signal"
	trace := r.Traces.Begin(operation, input.SubjectID, recordedAt)
	capturedBy := domain.UserZeroActor
	if options.CapturedBy != nil {
		capturedBy = *options.CapturedBy
	}
	policy := r.UncertaintyPolicy
	captured, rejections := domain.CaptureSignal(domain.CaptureSignalParams{
		Input:      input,
		CapturedBy: capturedBy,
		RecordedAt: recordedAt,
		ObservedAt: options.ObservedAt,
		Policy:     &policy,
	})
	if len(rejections) > 0 {
		trace.RecordStage(RecordStageParams{Stage: "S0", Operation: operation, At: recordedAt, InputRefs: []string{input.InputID}, Valid: false, Rejections: toExecutionRejections(rejections)})
		trace.Finish(recordedAt)
		return CaptureOutcome{}, rejections
	}

	report := domain.CheckSignal(captured.Signal)
	if !report.Valid {
		trace.RecordStage(RecordStageParams{Stage: "S0", Operation: operation, At: recordedAt, InputRefs: []string{input.InputID}, Valid: false, Rejections: toExecutionRejections(report.Rejections)})
		trace.Finish(recordedAt)
		return CaptureOutcome{}, report.Rejections
	}

	trace.RecordStage(RecordStageParams{
		Stage: "S0", Operation: operation, At: recordedAt,
		InputRefs: []string{input.InputID}, OutputRefs: []string{captured.Signal.ID},
		ChecksRun: report.ChecksRun, Valid: true,
	})
	trace.Finish(recordedAt)

	// Non-destructive capture: a corrected answer is a new signal that
	// supersedes the previous one; the previous signal stays readable.
	signal := captured.Signal
	var previous *domain.Signal
	if existing, ok := r.signals[signal.ID]; ok {
		previous = &existing
	} else {
		r.signalOrder = append(r.signalOrder, signal.ID)
	}
	r.signals[signal.ID] = signal
	r.persist(PersistedDocument{ID: signal.ID, Kind: "signal", SubjectID: signal.SubjectID, RecordedAt: recordedAt, Payload: signal})
	r.persist(PersistedDocument{ID: signal.ID + ":input", Kind: "signal-input", SubjectID: signal.SubjectID, RecordedAt: recordedAt, Payload: input})

	return CaptureOutcome{Signal: signal, Previous: previous}, nil
}

// ------------------------------------------------- S1→S2→S3→S4 decompose

type InvokeOptions struct {
	RequestedBy     *domain.ActorRef
	SessionRef      string
	InstructionText string
}

func (o InvokeOptions) actor() domain.ActorRef {
	if o.RequestedBy != nil {
		return *o.RequestedBy
	}
	return domain.UserZeroActor
}

func (r *CareerRuntime) DecomposeSignal(signalID string, options InvokeOptions) (DecomposeOutcome, []domain.Rejection) {
	r.mu.Lock()
	defer r.mu.Unlock()

	signal, ok := r.signals[signalID]
	if !ok {
		return DecomposeOutcome{}, []domain.Rejection{domain.NewRejection("S1.SIGNAL.NOT-FOUND", fmt.Sprintf("no captured signal %q in this runtime", signalID), "signalId")}
	}
	for _, id := range r.reprOrder {
		if existing := r.representations[id]; existing.SignalID == signal.ID {
			return DecomposeOutcome{}, []domain.Rejection{domain.NewRejection("S1.REPRESENTATION.EXISTS",
				fmt.Sprintf("signal %s already has representation %s; representations are versioned, not replaced", signal.ID, existing.ID), "signalId")}
		}
	}

	sessionRef := options.SessionRef
	if sessionRef == "" {
		sessionRef = signal.SessionRef
	}
	if sessionRef == "" {
		sessionRef = "session." + signal.SubjectID
	}
	request := domain.CapabilityRequest{
		CapabilityID:      domain.CapabilityDecompose1to3,
		CapabilityVersion: "0.1.0",
		SubjectID:         signal.SubjectID,
		RequestedBy:       options.actor(),
		SessionRef:        sessionRef,
		InputRefs:         []string{signal.ID},
		Parameters:        map[string]any{},
		RequestedAt:       r.clock.Now(),
	}
	descriptor := TransformationDescriptor{
		TransformationID:            "transformation.decompose-1to3",
		TransformationVersion:       "1.0.0",
		InputRepresentationVersion:  "1.0.0",
		OutputRepresentationVersion: "1.3.0",
		Deterministic:               true,
		RequiresModel:               false,
		DeclaredLosses:              []string{},
	}
	outcome, rejections := r.invoke(request, signal, descriptor)
	if len(rejections) > 0 {
		return DecomposeOutcome{}, rejections
	}

	representation, ok := outcome.Value.(domain.Representation1to3)
	if !ok {
		return DecomposeOutcome{}, []domain.Rejection{domain.NewRejection("S4.OUTPUT.SHAPE", "decomposition produced an invalid representation: unexpected output type", "output")}
	}
	if issues := domain.ValidateRepresentation(representation); len(issues) > 0 {
		return DecomposeOutcome{}, []domain.Rejection{domain.NewRejection("S4.OUTPUT.SHAPE", "decomposition produced an invalid representation: "+issues[0].Message, "output")}
	}
	if report := domain.CheckRepresentation(representation); !report.Valid {
		return DecomposeOutcome{}, report.Rejections
	}

	evidenceRefs := make([]string, len(representation.E.Bundle))
	for i, record := range representation.E.Bundle {
		evidenceRefs[i] = record.ID
	}
	uncertaintyRefs := make([]string, len(representation.U.Notes))
	for i, note := range representation.U.Notes {
		uncertaintyRefs[i] = note.ID
	}
	transformation, rejections := r.recordTransformationFor(outcome, recordParams{
		inputRefs:       []string{signal.ID},
		outputRefs:      []string{representation.ID},
		evidenceRefs:    evidenceRefs,
		uncertaintyRefs: uncertaintyRefs,
		declaredLosses:  outcome.HandlerOutput.DeclaredLosses,
		descriptor:      descriptor,
	})
	if len(rejections) > 0 {
		return DecomposeOutcome{}, rejections
	}

	r.representations[representation.ID] = representation
	r.reprOrder = append(r.reprOrder, representation.ID)
	r.persist(PersistedDocument{ID: representation.ID, Kind: "representation", SubjectID: representation.SubjectID, RecordedAt: representation.CreatedAt, Payload: representation})

	return DecomposeOutcome{Representation: representation, Transformation: transformation, Execution: outcome.Execution, Trace: outcome.Trace}, nil
}

// ---------------------------------------------- S1→S2→S3→S4 map pathways

func (r *CareerRuntime) MapPathways(representationIDs []string, options InvokeOptions) (PathwayOutcome, []domain.Rejection) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(representationIDs) == 0 {
		return PathwayOutcome{}, []domain.Rejection{domain.NewRejection("S2.PATHWAY.NO-REPRESENTATIONS",
			"pathway mapping requires at least one validated representation (S1 → S2)", "representationIds")}
	}
	representations := []domain.Representation1to3{}
	var rejections []domain.Rejection
	for _, id := range representationIDs {
		representation, ok := r.representations[id]
		if !ok {
			rejections = append(rejections, domain.NewRejection("S1.REPRESENTATION.NOT-FOUND", fmt.Sprintf("no representation %q in this runtime", id), "representationIds"))
			continue
		}
		representations = append(representations, representation)
	}
	if len(rejections) > 0 {
		return PathwayOutcome{}, rejections
	}
	subjectID := representations[0].SubjectID
	existingReviews := r.existingReviews(subjectID)

	inputRefs := make([]string, len(representations))
	for i, representation := range representations {
		inputRefs[i] = representation.ID
	}
	sessionRef := options.SessionRef
	if sessionRef == "" {
		sessionRef = "session." + subjectID
	}
	policy := domain.DefaultMappingPolicy
	request := domain.CapabilityRequest{
		CapabilityID:      domain.CapabilityMapPathways,
		CapabilityVersion: "0.1.0",
		SubjectID:         subjectID,
		RequestedBy:       options.actor(),
		SessionRef:        sessionRef,
		InputRefs:         inputRefs,
		Parameters:        map[string]any{"policyRef": policy.ID + "@" + policy.Version},
		RequestedAt:       r.clock.Now(),
	}
	payload := MapPathwaysPayload{Representations: representations, ExistingReviewStates: existingReviews}
	descriptor := TransformationDescriptor{
		TransformationID:            "transformation.map-pathways",
		TransformationVersion:       "1.0.0",
		InputRepresentationVersion:  "1.3.0",
		OutputRepresentationVersion: "1.0.0",
		Deterministic:               true,
		RequiresModel:               false,
		DeclaredLosses:              []string{},
	}
	outcome, rejections := r.invoke(request, payload, descriptor)
	if len(rejections) > 0 {
		return PathwayOutcome{}, rejections
	}

	set, ok := outcome.Value.(domain.PathwaySet)
	if !ok {
		return PathwayOutcome{}, []domain.Rejection{domain.NewRejection("S4.OUTPUT.SHAPE", "pathway mapping produced an invalid set: unexpected output type", "output")}
	}
	if issues := domain.ValidatePathwaySet(set); len(issues) > 0 {
		return PathwayOutcome{}, []domain.Rejection{domain.NewRejection("S4.OUTPUT.SHAPE", "pathway mapping produced an invalid set: "+issues[0].Message, "output")}
	}

	evidenceRefs := []string{}
	uncertaintyRefs := []string{}
	for _, pathway := range set.Pathways {
		evidenceRefs = append(evidenceRefs, pathway.EvidenceRefs...)
		for _, note := range pathway.Uncertainty {
			uncertaintyRefs = append(uncertaintyRefs, note.ID)
		}
	}
	transformation, rejections := r.recordTransformationFor(outcome, recordParams{
		inputRefs:       inputRefs,
		outputRefs:      []string{set.ID},
		evidenceRefs:    domain.UniqueStrings(evidenceRefs),
		uncertaintyRefs: domain.UniqueStrings(uncertaintyRefs),
		declaredLosses:  outcome.HandlerOutput.DeclaredLosses,
		descriptor:      descriptor,
	})
	if len(rejections) > 0 {
		return PathwayOutcome{}, rejections
	}

	if _, exists := r.pathwaySets[set.ID]; !exists {
		r.setOrder = append(r.setOrder, set.ID)
	}
	r.pathwaySets[set.ID] = set
	r.persist(PersistedDocument{ID: set.ID, Kind: "pathway-set", SubjectID: subjectID, RecordedAt: set.CreatedAt, Payload: set})

	return PathwayOutcome{PathwaySet: set, Transformation: transformation, Execution: outcome.Execution, Trace: outcome.Trace}, nil
}

// --------------------------------------------- model-assisted proposals (§5)

func (r *CareerRuntime) ProposeQuestions(representationID string, options InvokeOptions) (ProposalOutcome, []domain.Rejection) {
	r.mu.Lock()
	defer r.mu.Unlock()

	representation, ok := r.representations[representationID]
	if !ok {
		return ProposalOutcome{}, []domain.Rejection{domain.NewRejection("S1.REPRESENTATION.NOT-FOUND", fmt.Sprintf("no representation %q", representationID), "representationId")}
	}
	if r.model == nil {
		return ProposalOutcome{}, []domain.Rejection{domain.NewRejection("MODEL.NOT-CONFIGURED", "no model provider is registered; model-assisted workflows are unavailable", "model")}
	}
	sessionRef := options.SessionRef
	if sessionRef == "" {
		sessionRef = "session." + representation.SubjectID
	}
	request := domain.CapabilityRequest{
		CapabilityID:      domain.CapabilityProposeQuestions,
		CapabilityVersion: "0.1.0",
		SubjectID:         representation.SubjectID,
		RequestedBy:       options.actor(),
		SessionRef:        sessionRef,
		InputRefs:         []string{representation.ID},
		Parameters:        map[string]any{},
		RequestedAt:       r.clock.Now(),
	}
	instructionText := options.InstructionText
	if instructionText == "" {
		instructionText = "Propose up to three follow-up questions that would reduce the recorded uncertainty."
	}
	descriptor := TransformationDescriptor{
		TransformationID:            "transformation.propose-questions",
		TransformationVersion:       "1.0.0",
		InputRepresentationVersion:  "1.3.0",
		OutputRepresentationVersion: "1.0.0",
		Deterministic:               false,
		RequiresModel:               true,
		DeclaredLosses:              []string{},
	}
	outcome, rejections := r.invoke(request, ProposalPayload{
		Representation:     representation,
		InstructionRef:     "instruction.propose-questions.v1",
		InstructionVersion: "1.0.0",
		InstructionText:    instructionText,
	}, descriptor)
	if len(rejections) > 0 {
		return ProposalOutcome{}, rejections
	}
	output, ok := outcome.Value.(ProposalOutput)
	if !ok {
		return ProposalOutcome{}, []domain.Rejection{domain.NewRejection("S4.OUTPUT.SHAPE", "proposal capability produced an unexpected output", "output")}
	}

	proposal, rejections := domain.CreateProposalRecord(domain.CreateProposalParams{
		SubjectID:     representation.SubjectID,
		ProposalKind:  "candidate-question",
		ModelMetadata: output.ModelMetadata,
		Summary:       output.ProposalText,
		TargetRefs:    []string{representation.ID},
		RecordedAt:    r.clock.Now(),
		RecordedBy:    domain.SystemActor,
	})
	if len(rejections) > 0 {
		return ProposalOutcome{}, rejections
	}

	uncertaintyRefs := make([]string, len(proposal.Uncertainty))
	for i, note := range proposal.Uncertainty {
		uncertaintyRefs[i] = note.ID
	}
	metadata := output.ModelMetadata
	transformation, rejections := r.recordTransformationFor(outcome, recordParams{
		inputRefs:       []string{representation.ID},
		outputRefs:      []string{proposal.ID},
		evidenceRefs:    []string{},
		uncertaintyRefs: uncertaintyRefs,
		declaredLosses:  outcome.HandlerOutput.DeclaredLosses,
		descriptor:      descriptor,
		modelMetadata:   &metadata,
	})
	if len(rejections) > 0 {
		return ProposalOutcome{}, rejections
	}

	r.proposals = append(r.proposals, proposal)
	r.persist(PersistedDocument{ID: proposal.ID, Kind: "proposal", SubjectID: representation.SubjectID, RecordedAt: proposal.RecordedAt, Payload: proposal})

	return ProposalOutcome{Proposal: proposal, Transformation: transformation, Execution: outcome.Execution, Trace: outcome.Trace, Findings: output.Findings}, nil
}

// --------------------------------------------- 0 boundary: user decisions

type ReviewParams struct {
	PathwaySetID string
	PathwayID    string
	State        string
	DecidedBy    domain.ActorRef
	Rationale    string
}

type ReviewOutcome struct {
	PathwaySet domain.PathwaySet
	Decision   domain.UserDecisionRecord
}

func (r *CareerRuntime) SetPathwayReview(params ReviewParams) (ReviewOutcome, []domain.Rejection) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.pathwaySets[params.PathwaySetID]
	if !ok {
		return ReviewOutcome{}, []domain.Rejection{domain.NewRejection("S4.PATHWAY-SET.NOT-FOUND", fmt.Sprintf("no pathway set %q", params.PathwaySetID), "pathwaySetId")}
	}
	if !domain.IsUserActor(params.DecidedBy) {
		return ReviewOutcome{}, []domain.Rejection{domain.NewRejection("INV.PATHWAY.USER-AUTHORITY",
			fmt.Sprintf("pathway review must be decided by the user (0); actor %q may not decide", params.DecidedBy.Kind+":"+params.DecidedBy.ID), "decidedBy")}
	}

	decidedAt := r.clock.Now()
	updated, rejections := domain.SetPathwayReview(domain.SetPathwayReviewParams{
		Set:       existing,
		PathwayID: params.PathwayID,
		State:     params.State,
		DecidedBy: params.DecidedBy,
		DecidedAt: decidedAt,
		Rationale: params.Rationale,
	})
	if len(rejections) > 0 {
		return ReviewOutcome{}, rejections
	}

	kind := "explore-pathway"
	switch params.State {
	case "pursuing":
		kind = "pursue-pathway"
	case "not-a-fit":
		kind = "dismiss-pathway"
	case "deferred":
		kind = "defer-pathway"
	}
	decision, rejections := domain.CreateUserDecisionRecord(domain.CreateUserDecisionParams{
		SubjectID:       existing.SubjectID,
		DecisionKind:    kind,
		TargetRef:       params.PathwayID,
		DecidedAt:       decidedAt,
		DecidedBy:       params.DecidedBy,
		PresentationRef: existing.ID,
		Rationale:       params.Rationale,
	})
	if len(rejections) > 0 {
		return ReviewOutcome{}, rejections
	}

	r.pathwaySets[updated.ID] = updated
	r.decisions = append(r.decisions, decision)
	r.persist(PersistedDocument{ID: decision.ID, Kind: "user-decision", SubjectID: existing.SubjectID, RecordedAt: decidedAt, Payload: decision})
	return ReviewOutcome{PathwaySet: updated, Decision: decision}, nil
}

type CorrectionParams struct {
	RepresentationID      string
	Dimension             string
	Field                 string
	PreviousValueSummary  string
	CorrectedValueSummary string
	Reason                string
	CorrectedBy           domain.ActorRef
}

func (r *CareerRuntime) CorrectRepresentation(params CorrectionParams) (domain.CorrectionRecord, []domain.Rejection) {
	r.mu.Lock()
	defer r.mu.Unlock()

	representation, ok := r.representations[params.RepresentationID]
	if !ok {
		return domain.CorrectionRecord{}, []domain.Rejection{domain.NewRejection("S1.REPRESENTATION.NOT-FOUND", fmt.Sprintf("no representation %q", params.RepresentationID), "representationId")}
	}
	if !domain.IsUserActor(params.CorrectedBy) {
		return domain.CorrectionRecord{}, []domain.Rejection{domain.NewRejection("INV.PATHWAY.USER-AUTHORITY", "only the user (0) may correct their own representation", "correctedBy")}
	}
	correction, rejections := domain.CreateCorrectionRecord(domain.CreateCorrectionParams{
		SubjectID:             representation.SubjectID,
		TargetRef:             representation.ID,
		Dimension:             params.Dimension,
		Field:                 params.Field,
		PreviousValueSummary:  params.PreviousValueSummary,
		CorrectedValueSummary: params.CorrectedValueSummary,
		Reason:                params.Reason,
		RecordedAt:            r.clock.Now(),
		RecordedBy:            params.CorrectedBy,
	})
	if len(rejections) > 0 {
		return domain.CorrectionRecord{}, rejections
	}
	r.corrections = append(r.corrections, correction)
	r.persist(PersistedDocument{ID: correction.ID, Kind: "correction", SubjectID: representation.SubjectID, RecordedAt: correction.RecordedAt, Payload: correction})
	return correction, nil
}

type AssessmentRecordParams struct {
	SubjectID          string
	SessionRef         string
	SignalRefs         []string
	RepresentationRefs []string
	InstrumentRefs     []string
}

func (r *CareerRuntime) CreateAssessmentRecord(params AssessmentRecordParams) (domain.AssessmentRecord, []domain.Rejection) {
	r.mu.Lock()
	defer r.mu.Unlock()

	recordedAt := r.clock.Now()
	startedAt := recordedAt
	if len(params.SignalRefs) > 0 {
		if signal, ok := r.signals[params.SignalRefs[0]]; ok {
			startedAt = signal.CapturedAt
		}
	}
	record, rejections := domain.CreateAssessmentRecord(domain.CreateAssessmentRecordParams{
		SubjectID:          params.SubjectID,
		SessionRef:         params.SessionRef,
		StartedAt:          startedAt,
		RecordedAt:         recordedAt,
		RecordedBy:         domain.SystemActor,
		SignalRefs:         params.SignalRefs,
		RepresentationRefs: params.RepresentationRefs,
		InstrumentRefs:     params.InstrumentRefs,
	})
	if len(rejections) > 0 {
		return domain.AssessmentRecord{}, rejections
	}
	r.assessmentRecords = append(r.assessmentRecords, record)
	r.persist(PersistedDocument{ID: record.ID, Kind: "assessment-record", SubjectID: params.SubjectID, RecordedAt: recordedAt, Payload: record})
	return record, nil
}

func (r *CareerRuntime) StartSession(subjectID, promptSetRef string) (domain.SessionRecord, []domain.Rejection) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, rejections := domain.CreateSessionRecord(domain.CreateSessionRecordParams{
		SubjectID:    subjectID,
		StartedAt:    r.clock.Now(),
		RecordedBy:   domain.SystemActor,
		PromptSetRef: promptSetRef,
	})
	if len(rejections) > 0 {
		return domain.SessionRecord{}, rejections
	}
	r.sessions[session.ID] = session
	return session, nil
}

// ------------------------------------------------------------------ read side

func (r *CareerRuntime) GetSignal(id string) (domain.Signal, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	signal, ok := r.signals[id]
	return signal, ok
}

func (r *CareerRuntime) GetRepresentation(id string) (domain.Representation1to3, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	representation, ok := r.representations[id]
	return representation, ok
}

func (r *CareerRuntime) GetPathwaySet(id string) (domain.PathwaySet, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	set, ok := r.pathwaySets[id]
	return set, ok
}

// ListSignals lists signals in capture order; an empty subjectID lists all.
func (r *CareerRuntime) ListSignals(subjectID string) []domain.Signal {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.listSignals(subjectID)
}

func (r *CareerRuntime) listSignals(subjectID string) []domain.Signal {
	out := []domain.Signal{}
	for _, id := range r.signalOrder {
		if signal := r.signals[id]; subjectID == "" || signal.SubjectID == subjectID {
			out = append(out, signal)
		}
	}
	return out
}

func (r *CareerRuntime) ListRepresentations(subjectID string) []domain.Representation1to3 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.listRepresentations(subjectID)
}

func (r *CareerRuntime) listRepresentations(subjectID string) []domain.Representation1to3 {
	out := []domain.Representation1to3{}
	for _, id := range r.reprOrder {
		if representation := r.representations[id]; subjectID == "" || representation.SubjectID == subjectID {
			out = append(out, representation)
		}
	}
	return out
}

func (r *CareerRuntime) ListPathwaySets(subjectID string) []domain.PathwaySet {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.listPathwaySets(subjectID)
}

func (r *CareerRuntime) listPathwaySets(subjectID string) []domain.PathwaySet {
	out := []domain.PathwaySet{}
	for _, id := range r.setOrder {
		if set := r.pathwaySets[id]; subjectID == "" || set.SubjectID == subjectID {
			out = append(out, set)
		}
	}
	return out
}

func (r *CareerRuntime) ListTransformations() []domain.TransformationRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.TransformationRecord{}, r.transformations...)
}

func (r *CareerRuntime) ListExecutions() []ExecutionRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]ExecutionRecord{}, r.executions...)
}

func (r *CareerRuntime) ListDecisions() []domain.UserDecisionRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.UserDecisionRecord{}, r.decisions...)
}

func (r *CareerRuntime) ListCorrections() []domain.CorrectionRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.CorrectionRecord{}, r.corrections...)
}

func (r *CareerRuntime) ListProposals() []domain.ProposalRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.ProposalRecord{}, r.proposals...)
}

func (r *CareerRuntime) ListAssessmentRecords() []domain.AssessmentRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.AssessmentRecord{}, r.assessmentRecords...)
}

// Projection is a projection for the UI. It is derived state, never the
// domain model (§14).
func (r *CareerRuntime) Projection(subjectID string) RuntimeProjection {
	r.mu.Lock()
	defer r.mu.Unlock()
	representations := r.listRepresentations(subjectID)
	notes := 0
	for _, representation := range representations {
		notes += len(representation.U.Notes)
	}
	return RuntimeProjection{
		SubjectID:           subjectID,
		SignalCount:         len(r.listSignals(subjectID)),
		RepresentationCount: len(representations),
		PathwaySetCount:     len(r.listPathwaySets(subjectID)),
		TransformationCount: len(r.transformations),
		ExecutionCount:      len(r.executions),
		DecisionCount:       len(r.decisions),
		ProposalCount:       len(r.proposals),
		UncertaintyNotes:    notes,
		ContractVersions:    contractVersionSummary(),
		LastActivityAt:      r.clock.Now(),
	}
}

func (r *CareerRuntime) Now() string {
	return r.clock.Now()
}

// ------------------------------------------------------------------ internals

func (r *CareerRuntime) invoke(request domain.CapabilityRequest, payload any, descriptor TransformationDescriptor) (InvocationOutcome, []domain.Rejection) {
	return InvokeThroughPipeline(InvocationDeps{
		Registry:      r.Registry,
		Authorization: r.Authorization,
		Cloud107:      r.Cloud107,
		Artifacts:     r.Artifacts,
		Traces:        r.Traces,
		Clock:         r.clock,
		Hasher:        r.hasher,
		RuntimeID:     r.RuntimeID,
	}, InvokeParams{Request: request, Payload: payload, Transformation: descriptor, InputRefs: request.InputRefs})
}

type recordParams struct {
	inputRefs       []string
	outputRefs      []string
	evidenceRefs    []string
	uncertaintyRefs []string
	declaredLosses  []string
	descriptor      TransformationDescriptor
	modelMetadata   *domain.ProviderMetadata
}

func (r *CareerRuntime) recordTransformationFor(outcome InvocationOutcome, params recordParams) (domain.TransformationRecord, []domain.Rejection) {
	recorded, rejections := domain.RecordTransformation(domain.RecordTransformationParams[any]{
		Spec: domain.TransformationSpec{
			TransformationID:            params.descriptor.TransformationID,
			TransformationVersion:       params.descriptor.TransformationVersion,
			Name:                        params.descriptor.TransformationID,
			Description:                 "Executed through capability " + outcome.Execution.CapabilityRef,
			InputRepresentationVersion:  params.descriptor.InputRepresentationVersion,
			OutputRepresentationVersion: params.descriptor.OutputRepresentationVersion,
			Deterministic:               params.descriptor.Deterministic,
			RequiresModel:               params.descriptor.RequiresModel,
			DeclaredLosses:              params.declaredLosses,
		},
		Output:           outcome.Value,
		InputRefs:        params.inputRefs,
		OutputRefs:       params.outputRefs,
		EvidenceRefs:     params.evidenceRefs,
		UncertaintyRefs:  params.uncertaintyRefs,
		DeclaredLosses:   params.declaredLosses,
		HasLosses:        true,
		ExecutedAt:       outcome.Execution.FinishedAt,
		ExecutedBy:       domain.SystemActor,
		CapabilityRef:    outcome.Execution.CapabilityRef,
		AuthorizationRef: outcome.Authorization.ID,
		ModelMetadata:    params.modelMetadata,
	})
	if len(rejections) > 0 {
		return domain.TransformationRecord{}, rejections
	}
	r.transformations = append(r.transformations, recorded.Record)
	r.executions = append(r.executions, outcome.Execution)
	r.persist(PersistedDocument{ID: recorded.Record.ID, Kind: "transformation", RecordedAt: recorded.Record.ExecutedAt, Payload: recorded.Record})
	r.persist(PersistedDocument{ID: outcome.Execution.ID, Kind: "execution", RecordedAt: outcome.Execution.FinishedAt, Payload: outcome.Execution})
	r.persist(PersistedDocument{ID: outcome.Trace.TraceID, Kind: "trace", RecordedAt: outcome.Trace.FinishedAt, Payload: outcome.Trace})
	return recorded.Record, nil
}

// existingReviews returns the user's review states on the latest set for the
// subject, so that re-mapping keeps them.
func (r *CareerRuntime) existingReviews(subjectID string) map[string]string {
	sets := r.listPathwaySets(subjectID)
	if len(sets) == 0 {
		return nil
	}
	latest := sets[len(sets)-1]
	reviews := map[string]string{}
	for _, pathway := range latest.Pathways {
		if pathway.Review.State != "unreviewed" {
			reviews[pathway.PathwayID] = pathway.Review.State
		}
	}
	if len(reviews) == 0 {
		return nil
	}
	return reviews
}

func contractVersionSummary() map[string]string {
	return map[string]string{
		"domain":         "0.1.0",
		"representation": "1.3.0",
		"uncertainty":    "1.0.0",
		"pathway":        "1.0.0",
		"runtime":        RuntimeVersion,
	}
}
