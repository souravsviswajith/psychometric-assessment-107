// Package engine is the Career Assessment 107 runtime: the infrastructure side
// of the boundary (§8). It holds the S0–S4 pipeline, capability registry and
// authorization, execution records, artifacts, the persistence seam, the
// Cloud107 adapter and the model boundary. No career semantics live here.
package engine

import (
	"regexp"

	"career107/domain"
)

// These types are runtime/platform representations (S_3) and executed state
// (S_4). They carry no career semantics: a plan references a capability and a
// transformation; it never contains a career conclusion.

const RuntimeSchemaVersion = "0.1.0"

type EstimatedWork struct {
	InputBytes          int `json:"inputBytes"`
	RepresentationCount int `json:"representationCount"`
}

// ExecutionPlan is S_2 — the constrained plan. Everything the execution will
// need, decided before anything runs: capability, transformation, resolved
// inputs, policy versions, limits, and the authorization the invocation was
// bound to.
type ExecutionPlan struct {
	ID                string `json:"id"`
	SchemaVersion     string `json:"schemaVersion"`
	SubjectID         string `json:"subjectId"`
	CapabilityID      string `json:"capabilityId"`
	CapabilityVersion string `json:"capabilityVersion"`
	TransformationRef string `json:"transformationRef"`
	// S_1 inputs, resolved and validated before planning is allowed to finish.
	RepresentationRefs []string      `json:"representationRefs"`
	SignalRefs         []string      `json:"signalRefs"`
	PolicyRefs         []string      `json:"policyRefs"`
	ParametersDigest   domain.Digest `json:"parametersDigest"`
	MaxInputBytes      int           `json:"maxInputBytes"`
	EstimatedWork      EstimatedWork `json:"estimatedWork"`
	AuthorizationRef   string        `json:"authorizationRef"`
	PlannedAt          string        `json:"plannedAt"`
	// Declared before execution: what this plan will not do.
	Constraints []string `json:"constraints"`
}

type InvocationLimits struct {
	MaxInputBytes  int  `json:"maxInputBytes"`
	MaxWallClockMs int  `json:"maxWallClockMs"`
	AllowNetwork   bool `json:"allowNetwork"`
}

// RuntimeInvocation is S_3 — the platform/runtime representation of the work
// to be executed.
type RuntimeInvocation struct {
	ID            string `json:"id"`
	SchemaVersion string `json:"schemaVersion"`
	PlanRef       string `json:"planRef"`
	CapabilityRef string `json:"capabilityRef"`
	// Payload is held as a digest-verified artifact, never inline code.
	PayloadRef    string        `json:"payloadRef"`
	PayloadDigest domain.Digest `json:"payloadDigest"`
	RequestedAt   string        `json:"requestedAt"`
	RuntimeID     string        `json:"runtimeId"`
	// Hard limits the execution substrate must respect.
	Limits InvocationLimits `json:"limits"`
}

var ExecutionStatuses = []string{"pending", "succeeded", "failed", "rejected"}

type Executor struct {
	RuntimeID string `json:"runtimeId"`
	AdapterID string `json:"adapterId"`
	// Infrastructure-only metadata; never career semantics.
	Infrastructure map[string]string `json:"infrastructure"`
}

type ExecutionRejection struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

type ExecutionRecord struct {
	ID                string               `json:"id"`
	SchemaVersion     string               `json:"schemaVersion"`
	InvocationRef     string               `json:"invocationRef"`
	PlanRef           string               `json:"planRef"`
	CapabilityRef     string               `json:"capabilityRef"`
	Status            string               `json:"status"`
	StartedAt         string               `json:"startedAt"`
	FinishedAt        string               `json:"finishedAt"`
	Executor          Executor             `json:"executor"`
	ArtifactRefs      []string             `json:"artifactRefs"`
	OutputRefs        []string             `json:"outputRefs"`
	Rejections        []ExecutionRejection `json:"rejections"`
	TransformationRef string               `json:"transformationRef,omitempty"`
}

type ArtifactIntegrity struct {
	Verified   bool   `json:"verified"`
	VerifiedAt string `json:"verifiedAt,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

type Artifact struct {
	ID         string        `json:"id"`
	Digest     domain.Digest `json:"digest"`
	MediaType  string        `json:"mediaType"`
	ByteLength int           `json:"byteLength"`
	CreatedAt  string        `json:"createdAt"`
	CreatedBy  string        `json:"createdBy"`
	SubjectID  string        `json:"subjectId,omitempty"`
	// Artifact content is stored out of band; the store returns it by reference.
	StorageRef string `json:"storageRef"`
	// Digest strength is recorded, so a weak integrity check cannot look strong.
	Integrity ArtifactIntegrity `json:"integrity"`
}

// --------------------------------------------------------------- validation

var (
	semver  = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	isoTime = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d{1,3})?Z$`)
	digestV = regexp.MustCompile(`^[0-9a-f]{16,128}$`)
)

type validator struct{ issues []domain.Issue }

func (v *validator) add(path, message string) {
	v.issues = append(v.issues, domain.Issue{Path: path, Message: message})
}

func (v *validator) str(path, value string, min, max int) {
	n := domain.JSLength(value)
	if n < min {
		v.add(path, "Too small: expected string to have >="+itoa(min)+" characters")
	} else if max >= 0 && n > max {
		v.add(path, "Too big: expected string to have <="+itoa(max)+" characters")
	}
}

func (v *validator) list(path string, values []string, min, max int) {
	if values == nil {
		v.add(path, "Invalid input: expected array, received undefined")
		return
	}
	if len(values) < min {
		v.add(path, "Too small: expected array to have >="+itoa(min)+" items")
	} else if max >= 0 && len(values) > max {
		v.add(path, "Too big: expected array to have <="+itoa(max)+" items")
	}
	for i, value := range values {
		v.str(path+"."+itoa(i), value, 1, -1)
	}
}

func (v *validator) re(path, value string, pattern *regexp.Regexp, message string) {
	if !pattern.MatchString(value) {
		v.add(path, message)
	}
}

func (v *validator) id(path, value string, kind interface{ Valid(string) bool }, name, prefix string) {
	if !kind.Valid(value) {
		v.add(path, name+" must match ^"+prefix+"_<slug> (lowercase slug, 2-80 chars)")
	}
}

func (v *validator) digest(path string, d domain.Digest) {
	for _, issue := range domain.ValidateDigest(d) {
		p := path
		if issue.Path != "" {
			p = path + "." + issue.Path
		}
		v.add(p, issue.Message)
	}
}

func (v *validator) semver(path, value string) {
	v.re(path, value, semver, "semantic version must be MAJOR.MINOR.PATCH")
}

func (v *validator) iso(path, value string) {
	v.re(path, value, isoTime, "timestamps must be ISO-8601 UTC with Z suffix")
}

func ValidateExecutionPlan(p ExecutionPlan) []domain.Issue {
	v := &validator{}
	v.id("id", p.ID, domain.PlanIDKind, "PlanId", "plan")
	v.semver("schemaVersion", p.SchemaVersion)
	v.id("subjectId", p.SubjectID, domain.SubjectIDKind, "SubjectId", "sub")
	if !domain.ValidCapabilityID(p.CapabilityID) {
		v.add("capabilityId", "capability id must be a dotted lowercase namespace, e.g. career.assessment.decompose-1to3")
	}
	v.semver("capabilityVersion", p.CapabilityVersion)
	v.str("transformationRef", p.TransformationRef, 1, 160)
	v.list("representationRefs", p.RepresentationRefs, 1, 64)
	v.list("signalRefs", p.SignalRefs, 0, 128)
	v.list("policyRefs", p.PolicyRefs, 0, 16)
	for i, ref := range p.PolicyRefs {
		v.str("policyRefs."+itoa(i), ref, 1, 160)
	}
	v.digest("parametersDigest", p.ParametersDigest)
	if p.MaxInputBytes <= 0 {
		v.add("maxInputBytes", "Too small: expected number to be >0")
	}
	if p.EstimatedWork.InputBytes < 0 || p.EstimatedWork.RepresentationCount < 0 {
		v.add("estimatedWork", "Too small: expected number to be >=0")
	}
	v.str("authorizationRef", p.AuthorizationRef, 1, 160)
	v.iso("plannedAt", p.PlannedAt)
	v.list("constraints", p.Constraints, 0, 8)
	for i, constraint := range p.Constraints {
		v.str("constraints."+itoa(i), constraint, 1, 300)
	}
	return v.issues
}

func ValidateRuntimeInvocation(r RuntimeInvocation) []domain.Issue {
	v := &validator{}
	v.id("id", r.ID, domain.PlatformRepIDKind, "PlatformRepId", "plat")
	v.semver("schemaVersion", r.SchemaVersion)
	v.id("planRef", r.PlanRef, domain.PlanIDKind, "PlanId", "plan")
	v.str("capabilityRef", r.CapabilityRef, 1, 160)
	v.str("payloadRef", r.PayloadRef, 1, 200)
	v.digest("payloadDigest", r.PayloadDigest)
	v.iso("requestedAt", r.RequestedAt)
	v.str("runtimeId", r.RuntimeID, 1, 120)
	if r.Limits.MaxInputBytes <= 0 {
		v.add("limits.maxInputBytes", "Too small: expected number to be >0")
	}
	if r.Limits.MaxWallClockMs <= 0 {
		v.add("limits.maxWallClockMs", "Too small: expected number to be >0")
	}
	if r.Limits.AllowNetwork {
		v.add("limits.allowNetwork", "Invalid input: expected false")
	}
	return v.issues
}

func ValidateExecutionRecord(e ExecutionRecord) []domain.Issue {
	v := &validator{}
	v.id("id", e.ID, domain.ExecutionIDKind, "ExecutionId", "exe")
	v.semver("schemaVersion", e.SchemaVersion)
	v.id("invocationRef", e.InvocationRef, domain.PlatformRepIDKind, "PlatformRepId", "plat")
	v.id("planRef", e.PlanRef, domain.PlanIDKind, "PlanId", "plan")
	v.str("capabilityRef", e.CapabilityRef, 1, 160)
	if !domain.Contains(ExecutionStatuses, e.Status) {
		v.add("status", `Invalid option: expected one of "pending"|"succeeded"|"failed"|"rejected"`)
	}
	v.iso("startedAt", e.StartedAt)
	v.iso("finishedAt", e.FinishedAt)
	v.str("executor.runtimeId", e.Executor.RuntimeID, 1, 120)
	v.str("executor.adapterId", e.Executor.AdapterID, 1, 120)
	v.list("artifactRefs", e.ArtifactRefs, 0, 64)
	v.list("outputRefs", e.OutputRefs, 0, 64)
	for i, rejection := range e.Rejections {
		v.str("rejections."+itoa(i)+".code", rejection.Code, 1, 80)
		v.str("rejections."+itoa(i)+".message", rejection.Message, 1, 600)
		if rejection.Path != "" {
			v.str("rejections."+itoa(i)+".path", rejection.Path, 0, 200)
		}
	}
	if e.TransformationRef != "" {
		v.str("transformationRef", e.TransformationRef, 1, 160)
	}
	return v.issues
}

func ValidateArtifact(a Artifact) []domain.Issue {
	v := &validator{}
	v.id("id", a.ID, domain.ArtifactIDKind, "ArtifactId", "art")
	v.digest("digest", a.Digest)
	v.str("mediaType", a.MediaType, 1, 80)
	if a.ByteLength < 0 {
		v.add("byteLength", "Too small: expected number to be >=0")
	}
	v.iso("createdAt", a.CreatedAt)
	v.str("createdBy", a.CreatedBy, 1, 120)
	if a.SubjectID != "" {
		v.id("subjectId", a.SubjectID, domain.SubjectIDKind, "SubjectId", "sub")
	}
	v.str("storageRef", a.StorageRef, 1, 200)
	if a.Integrity.VerifiedAt != "" {
		v.iso("integrity.verifiedAt", a.Integrity.VerifiedAt)
	}
	if a.Integrity.Notes != "" {
		v.str("integrity.notes", a.Integrity.Notes, 0, 300)
	}
	return v.issues
}

// ------------------------------------------------------------ Cloud107 contract
//
// The runtime calls this interface; the deployment supplies the
// implementation (in-process for development, a queue/worker substrate in
// production). Career semantics never cross this boundary (§8).

type WorkOutcome struct {
	OK         bool
	Value      any
	OutputRefs []string
	Rejections []ExecutionRejection
}

type ExecutionContext struct {
	RuntimeID  string
	SubjectRef string
	Parameters map[string]any
	// Digest-verified payload produced by the runtime at S_3.
	Payload any
	// The work Cloud107 is asked to run. The 107 runtime supplies it; Cloud107
	// schedules and executes it without interpreting it.
	Execute func() WorkOutcome
}

type Cloud107Result struct {
	Status         string
	ExecutedAt     string
	Result         any
	HasResult      bool
	ArtifactRefs   []string
	Infrastructure map[string]string
	Notes          []string
}

type Cloud107Adapter interface {
	AdapterID() string
	Invoke(invocation RuntimeInvocation, ctx ExecutionContext) Cloud107Result
}

// ------------------------------------------------------------------ traces

var StageIDs = []string{"S0", "S1", "S2", "S3", "S4"}

type StageEnvelope struct {
	Stage      string               `json:"stage"`
	Operation  string               `json:"operation"`
	At         string               `json:"at"`
	InputRefs  []string             `json:"inputRefs"`
	OutputRefs []string             `json:"outputRefs"`
	ChecksRun  int                  `json:"checksRun"`
	Valid      bool                 `json:"valid"`
	Rejections []ExecutionRejection `json:"rejections"`
}

// PipelineTrace is the S_0 → S_4 path actually taken, for verification (§21).
type PipelineTrace struct {
	TraceID    string          `json:"traceId"`
	Operation  string          `json:"operation"`
	SubjectID  string          `json:"subjectId"`
	Stages     []StageEnvelope `json:"stages"`
	StartedAt  string          `json:"startedAt"`
	FinishedAt string          `json:"finishedAt"`
}

func TraceCoversStages(trace PipelineTrace, stages []string) bool {
	seen := map[string]bool{}
	for _, stage := range trace.Stages {
		seen[stage.Stage] = true
	}
	for _, stage := range stages {
		if !seen[stage] {
			return false
		}
	}
	return true
}

func StageOrder(trace PipelineTrace) []string {
	order := make([]string, len(trace.Stages))
	for i, stage := range trace.Stages {
		order[i] = stage.Stage
	}
	return order
}

// RuntimeProjection is a projection of runtime state for the UI (§14).
// Derived state: the UI renders this, but it is not the domain model.
type RuntimeProjection struct {
	SubjectID           string            `json:"subjectId"`
	SignalCount         int               `json:"signalCount"`
	RepresentationCount int               `json:"representationCount"`
	PathwaySetCount     int               `json:"pathwaySetCount"`
	TransformationCount int               `json:"transformationCount"`
	ExecutionCount      int               `json:"executionCount"`
	DecisionCount       int               `json:"decisionCount"`
	ProposalCount       int               `json:"proposalCount"`
	UncertaintyNotes    int               `json:"uncertaintyNotes"`
	ContractVersions    map[string]string `json:"contractVersions"`
	LastActivityAt      string            `json:"lastActivityAt"`
}

func toExecutionRejections(rejections []domain.Rejection) []ExecutionRejection {
	out := make([]ExecutionRejection, len(rejections))
	for i, r := range rejections {
		out[i] = ExecutionRejection{Code: r.Code, Message: r.Message}
	}
	return out
}
