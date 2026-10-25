package engine

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"career107/domain"
)

// The Cloud107 boundary (§8).
//
// Career Assessment asks Cloud107 to execute a plan. Cloud107 owns workload
// execution, scheduling, artifact handling, and execution state. It receives
// a capability reference, a digest-verified payload, and limits — never career
// semantics, never a scoring rule, never a decision about a person.
//
// The in-process adapter below is a development substrate: it executes the
// handler that the runtime registered for the capability. A production
// deployment substitutes a queue/worker adapter implementing the same
// interface; nothing else in the system changes.

type InProcessCloud107Adapter struct {
	RuntimeID string
	// Fixed wall-clock cost, in milliseconds, reported as infrastructure metadata.
	SimulatedLatencyMs int64
}

func (a *InProcessCloud107Adapter) AdapterID() string { return "cloud107.in-process.v1" }

func (a *InProcessCloud107Adapter) runtimeID() string {
	if a.RuntimeID == "" {
		return "cloud107-in-process"
	}
	return a.RuntimeID
}

func (a *InProcessCloud107Adapter) Invoke(invocation RuntimeInvocation, ctx ExecutionContext) Cloud107Result {
	started := time.Now()

	// Cloud107 executes work; it does not interpret it.
	if refusal := checkInvocationLimits(invocation, ctx); refusal != "" {
		return Cloud107Result{
			Status:         "failed",
			ExecutedAt:     domain.FormatIso(time.Now()),
			ArtifactRefs:   []string{},
			Infrastructure: map[string]string{"runtimeId": a.runtimeID(), "adapterId": a.AdapterID()},
			Notes:          []string{refusal},
		}
	}

	outcome := runGuarded(ctx.Execute)

	latency := time.Since(started).Milliseconds() + a.SimulatedLatencyMs
	infrastructure := map[string]string{
		"runtimeId":   a.runtimeID(),
		"adapterId":   a.AdapterID(),
		"wallClockMs": strconv.FormatInt(latency, 10),
	}
	if !outcome.OK {
		notes := make([]string, len(outcome.Rejections))
		for i, rejection := range outcome.Rejections {
			notes[i] = rejection.Message
		}
		return Cloud107Result{Status: "failed", ExecutedAt: domain.FormatIso(time.Now()), ArtifactRefs: []string{}, Infrastructure: infrastructure, Notes: notes}
	}
	return Cloud107Result{
		Status:     "succeeded",
		ExecutedAt: domain.FormatIso(time.Now()),
		Result:     outcome.Value,
		HasResult:  outcome.Value != nil,
		// The substrate executes work; artifact storage belongs to the runtime,
		// which stores the result under a digest of its own.
		ArtifactRefs:   []string{},
		Infrastructure: infrastructure,
		Notes:          []string{"executed in-process; no career semantics crossed the Cloud107 boundary"},
	}
}

// runGuarded turns a panic inside a handler into a failed outcome, so one bad
// payload cannot take the whole runtime down.
func runGuarded(execute func() WorkOutcome) (outcome WorkOutcome) {
	defer func() {
		if recovered := recover(); recovered != nil {
			outcome = WorkOutcome{OK: false, Rejections: []ExecutionRejection{{Code: "S4.EXECUTION.PANIC", Message: fmt.Sprintf("capability handler failed: %v", recovered)}}}
		}
	}()
	return execute()
}

func checkInvocationLimits(invocation RuntimeInvocation, ctx ExecutionContext) string {
	if invocation.Limits.AllowNetwork {
		return "refusing invocation: network access is not permitted for assessment work"
	}
	raw, err := json.Marshal(ctx.Payload)
	if err != nil {
		return "refusing invocation: payload is not serialisable data"
	}
	if approx := domain.JSLength(string(raw)); approx > invocation.Limits.MaxInputBytes {
		return fmt.Sprintf("payload of %d bytes exceeds the planned limit of %d bytes", approx, invocation.Limits.MaxInputBytes)
	}
	return ""
}

// Cloud107ForbiddenPayloadKeys are what Cloud107 may never receive. Encoded as
// a check so that the boundary is testable rather than merely documented.
var Cloud107ForbiddenPayloadKeys = []string{
	"scoringRule",
	"careerDecision",
	"recommendation",
	"ranking",
	"bestCareer",
	"careerScore",
	"psychometricModel",
}

func ScanPayloadForCareerSemantics(payload any, path string) []string {
	if path == "" {
		path = "$"
	}
	findings := []string{}
	var walk func(node any, current string)
	walk = func(node any, current string) {
		switch v := node.(type) {
		case map[string]any:
			for _, key := range domain.ObjectKeys(v) {
				if domain.Contains(Cloud107ForbiddenPayloadKeys, key) {
					findings = append(findings, current+"."+key)
				}
				walk(v[key], current+"."+key)
			}
		case []any:
			for i, item := range v {
				walk(item, current+"."+strconv.Itoa(i))
			}
		}
	}
	walk(domain.ToGeneric(payload), path)
	return findings
}

// --------------------------------------------------------------- persistence
//
// The storage seam (§16). Everything the runtime stores goes through this
// interface. The in-memory implementation is the default; a durable
// implementation stores the same documents, with the same append-only
// semantics, and requires no change to the domain.

type PersistedDocument struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	SubjectID  string `json:"subjectId,omitempty"`
	RecordedAt string `json:"recordedAt"`
	Payload    any    `json:"payload"`
}

type ListOptions struct {
	SubjectID string
	Limit     int
}

type CareerPersistence interface {
	PersistenceID() string
	// Put is append-only by contract: overwriting a stored document is refused.
	Put(document PersistedDocument) error
	Get(kind, id string) (PersistedDocument, bool)
	List(kind string, options ListOptions) []PersistedDocument
	AppendOnly() bool
}

// InMemoryCareerPersistence enforces the same contract as a durable store:
// documents are stored once and never overwritten.
type InMemoryCareerPersistence struct {
	mu        sync.Mutex
	order     []string
	documents map[string]PersistedDocument
	Refusals  []string
}

func NewInMemoryCareerPersistence() *InMemoryCareerPersistence {
	return &InMemoryCareerPersistence{documents: map[string]PersistedDocument{}}
}

func (p *InMemoryCareerPersistence) PersistenceID() string { return "persistence.in-memory.v1" }
func (p *InMemoryCareerPersistence) AppendOnly() bool      { return true }

func (p *InMemoryCareerPersistence) Put(document PersistedDocument) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := document.Kind + ":" + document.ID
	if _, exists := p.documents[key]; exists {
		refusal := fmt.Sprintf("refused to overwrite %s: an existing document is never replaced (versioned records are appended instead)", key)
		p.Refusals = append(p.Refusals, refusal)
		return nil
	}
	p.documents[key] = document
	p.order = append(p.order, key)
	return nil
}

func (p *InMemoryCareerPersistence) Get(kind, id string) (PersistedDocument, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	document, ok := p.documents[kind+":"+id]
	return document, ok
}

func (p *InMemoryCareerPersistence) List(kind string, options ListOptions) []PersistedDocument {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := []PersistedDocument{}
	for _, key := range p.order {
		document := p.documents[key]
		if document.Kind == kind && (options.SubjectID == "" || document.SubjectID == options.SubjectID) {
			out = append(out, document)
		}
	}
	if options.Limit > 0 && len(out) > options.Limit {
		out = out[:options.Limit]
	}
	return out
}

func (p *InMemoryCareerPersistence) Count(kind string) int {
	return len(p.List(kind, ListOptions{}))
}

// ---------------------------------------------------------------- traces
//
// The S_0 → S_4 stage trace. §21 asks for verification of the actual path, so
// the runtime records the stages it really took, with the checks that ran and
// the references that moved between stages.

type TraceLog struct {
	mu     sync.Mutex
	traces []PipelineTrace
}

func (l *TraceLog) Begin(operation, subjectID, at string) *TraceBuilder {
	return &TraceBuilder{
		operation: operation,
		subjectID: subjectID,
		startedAt: at,
		traceID:   domain.MintID("trf", map[string]any{"kind": "trace", "operation": operation, "at": at, "subjectId": subjectID}),
		stages:    []StageEnvelope{},
		commit: func(trace PipelineTrace) {
			l.mu.Lock()
			defer l.mu.Unlock()
			l.traces = append(l.traces, trace)
		},
	}
}

func (l *TraceLog) All() []PipelineTrace {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]PipelineTrace{}, l.traces...)
}

func (l *TraceLog) Last() (PipelineTrace, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.traces) == 0 {
		return PipelineTrace{}, false
	}
	return l.traces[len(l.traces)-1], true
}

func (l *TraceLog) ByOperation(operation string) []PipelineTrace {
	out := []PipelineTrace{}
	for _, trace := range l.All() {
		if trace.Operation == operation {
			out = append(out, trace)
		}
	}
	return out
}

type RecordStageParams struct {
	Stage      string
	Operation  string
	At         string
	InputRefs  []string
	OutputRefs []string
	ChecksRun  int
	Valid      bool
	Rejections []ExecutionRejection
}

type TraceBuilder struct {
	operation string
	subjectID string
	startedAt string
	traceID   string
	stages    []StageEnvelope
	commit    func(PipelineTrace)
}

func (b *TraceBuilder) RecordStage(params RecordStageParams) {
	rejections := make([]ExecutionRejection, len(params.Rejections))
	for i, r := range params.Rejections {
		rejections[i] = ExecutionRejection{Code: r.Code, Message: r.Message}
	}
	b.stages = append(b.stages, StageEnvelope{
		Stage:      params.Stage,
		Operation:  params.Operation,
		At:         params.At,
		InputRefs:  domain.CopyStrings(params.InputRefs),
		OutputRefs: domain.CopyStrings(params.OutputRefs),
		ChecksRun:  params.ChecksRun,
		Valid:      params.Valid,
		Rejections: rejections,
	})
}

func (b *TraceBuilder) Finish(finishedAt string) PipelineTrace {
	trace := PipelineTrace{
		TraceID:    b.traceID,
		Operation:  b.operation,
		SubjectID:  b.subjectID,
		Stages:     append([]StageEnvelope{}, b.stages...),
		StartedAt:  b.startedAt,
		FinishedAt: finishedAt,
	}
	b.commit(trace)
	return trace
}
