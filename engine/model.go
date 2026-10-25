package engine

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"career107/domain"
)

// The model boundary (§5).
//
// A language model may assist: extract semantics, propose questions, explain
// a representation. It is never the source of truth for psychometric
// validity, evidence provenance, assessment validity, career decisions,
// authorization, execution state, user identity, or history.
//
// Everything a model produces enters the system as a proposal with full
// metadata, and requires an explicit user decision before it can be used.

type ModelGenerationRequest struct {
	// Stable instruction reference, e.g. "instruction.propose-questions.v1".
	InstructionRef     string
	InstructionVersion string
	InstructionText    string
	// Reference to the representation the model is allowed to see.
	InputRef       string
	SubjectID      string
	MaxOutputChars int
}

type ModelGenerationResult struct {
	Provider           string
	Model              string
	ModelVersion       string
	InstructionRef     string
	InstructionVersion string
	InstructionText    string
	InputRef           string
	OutputRef          string
	Text               string
	GeneratedAt        string
	Determinism        string
	Temperature        *float64
	// What the model itself says it is unsure about — mandatory, never omitted.
	Uncertainty string
}

type ModelProvider interface {
	ProviderID() string
	Supports() []string
	Generate(request ModelGenerationRequest) (ModelGenerationResult, []domain.Rejection)
}

func ProviderMetadataFrom(result ModelGenerationResult, hasher domain.Hasher) domain.ProviderMetadata {
	if hasher == nil {
		hasher = domain.FNV1a64Hasher
	}
	digest := domain.DigestOf(hasher, result.InstructionText, fmt.Sprintf("instruction:%s@%s", result.InstructionRef, result.InstructionVersion))
	return domain.ProviderMetadata{
		Provider:           result.Provider,
		Model:              result.Model,
		ModelVersion:       result.ModelVersion,
		InstructionVersion: result.InstructionRef + "@" + result.InstructionVersion,
		InputRef:           result.InputRef,
		OutputRef:          result.OutputRef,
		Uncertainty:        result.Uncertainty,
		GeneratedAt:        result.GeneratedAt,
		Determinism:        result.Determinism,
		InstructionDigest:  &digest,
		Temperature:        result.Temperature,
	}
}

// DeterministicStubModelProvider is the development substrate. It is
// deliberately not a real model client: it makes the contract executable
// (metadata completeness, proposal-only semantics) without pretending that a
// stub produces useful career prose. A production deployment registers a real
// provider implementing ModelProvider; the rest of the system is unchanged.
type DeterministicStubModelProvider struct {
	ModelVersion string
	Uncertainty  string
}

func (p *DeterministicStubModelProvider) ProviderID() string { return "stub.local" }

func (p *DeterministicStubModelProvider) Supports() []string {
	return []string{domain.CapabilityProposeQuestions, domain.CapabilityExplainRepresentation}
}

func (p *DeterministicStubModelProvider) Generate(request ModelGenerationRequest) (ModelGenerationResult, []domain.Rejection) {
	if strings.TrimSpace(request.InputRef) == "" {
		return ModelGenerationResult{}, []domain.Rejection{domain.NewRejection("MODEL.INPUT-REF", "model generation requires an input reference", "inputRef")}
	}
	if strings.TrimSpace(request.InstructionText) == "" {
		return ModelGenerationResult{}, []domain.Rejection{domain.NewRejection("MODEL.INSTRUCTION", "model generation requires an instruction", "instructionText")}
	}
	text := "Proposed follow-up questions (model-generated, unverified): " +
		"1) Which observation in this record would you most want to check again in six months? " +
		"2) Which context field, if it changed, would make you read this observation differently?"
	modelVersion := p.ModelVersion
	if modelVersion == "" {
		modelVersion = "0.0.1-stub"
	}
	uncertainty := p.Uncertainty
	if uncertainty == "" {
		uncertainty = "Stub provider: the text is a placeholder that demonstrates the proposal contract, not a model judgement."
	}
	if domain.JSLength(text) > request.MaxOutputChars {
		text = domain.JSSlice(text, 0, request.MaxOutputChars)
	}
	return ModelGenerationResult{
		Provider:           p.ProviderID(),
		Model:              "stub-model",
		ModelVersion:       modelVersion,
		InstructionRef:     request.InstructionRef,
		InstructionVersion: request.InstructionVersion,
		InstructionText:    request.InstructionText,
		InputRef:           request.InputRef,
		OutputRef:          fmt.Sprintf("model-output:%s:%s", request.InputRef, request.InstructionRef),
		Text:               text,
		GeneratedAt:        domain.FormatIso(time.Now()),
		Determinism:        "deterministic",
		Uncertainty:        uncertainty,
	}, nil
}

// ------------------------------------------------------------------ safety
//
// Safe handling of model-generated content (§17):
//   - model output is data, never instruction and never code;
//   - output is normalised (control characters removed, length bounded,
//     scheme-bearing links neutralised) before it reaches a user or a record;
//   - content that tries to alter the system's instructions is flagged, not
//     obeyed;
//   - a proposal can never be used as evidence, and never becomes an
//     observation.

const MaxProposalChars = 4000

// InjectionMarkers are phrases that indicate an injected instruction rather
// than career content.
var InjectionMarkers = []string{
	"ignore previous instructions",
	"ignore all previous",
	"disregard the above",
	"you are now",
	"system prompt",
	"reveal your instructions",
	"override the",
	"execute the following",
	"run the following code",
	"delete all",
}

func ControlCharacterCount(text string) int {
	count := 0
	for _, r := range text {
		if (r < 0x20 && r != 0x0a && r != 0x09) || r == 0x7f {
			count++
		}
	}
	return count
}

type SanitisedText struct {
	Text     string
	Removals []string
}

var controlChars = regexp.MustCompile(`[\x00-\x08\x0b-\x1f\x7f]`)
var schemeLinks = regexp.MustCompile(`(?i)\[([^\]]*)\]\((javascript|data|file|vbscript):[^)]*\)`)

func SanitiseModelText(input string, maxChars int) SanitisedText {
	if maxChars <= 0 {
		maxChars = MaxProposalChars
	}
	removals := []string{}
	withoutControl := controlChars.ReplaceAllString(input, "")
	if domain.JSLength(withoutControl) != domain.JSLength(input) {
		removals = append(removals, "control characters removed")
	}
	withoutLinks := schemeLinks.ReplaceAllString(withoutControl, "$1")
	if withoutLinks != withoutControl {
		removals = append(removals, "scheme-bearing links neutralised")
	}
	bounded := withoutLinks
	if domain.JSLength(withoutLinks) > maxChars {
		bounded = domain.JSSlice(withoutLinks, 0, maxChars)
		removals = append(removals, fmt.Sprintf("truncated to %d characters", maxChars))
	}
	return SanitisedText{Text: strings.TrimSpace(bounded), Removals: removals}
}

type ModelOutputAssessment struct {
	Sanitised          string
	Removals           []string
	Findings           []domain.Rejection
	InjectionSuspected bool
}

// AssessModelOutput assesses model output before it is stored or displayed.
// Findings do not "sanitise away" a suspected injection: the content is
// retained for review, and the finding travels with it.
func AssessModelOutput(input string, maxChars int) ModelOutputAssessment {
	sanitised := SanitiseModelText(input, maxChars)
	lowered := strings.ToLower(sanitised.Text)
	findings := []domain.Rejection{}
	suspected := false
	for _, marker := range InjectionMarkers {
		if strings.Contains(lowered, marker) {
			suspected = true
			findings = append(findings, domain.NewRejection("MODEL.OUTPUT.INJECTION-SUSPECTED",
				fmt.Sprintf("model output contains instruction-like content (%q); it is retained as a proposal and never executed", marker), "summary"))
		}
	}
	if strings.TrimSpace(sanitised.Text) == "" {
		findings = append(findings, domain.NewRejection("MODEL.OUTPUT.EMPTY", "model output is empty after normalisation", "summary"))
	}
	return ModelOutputAssessment{Sanitised: sanitised.Text, Removals: sanitised.Removals, Findings: findings, InjectionSuspected: suspected}
}

// AssertProposalOnlyUse: a proposal is never evidence and never an
// observation (§5). Called wherever a proposal is about to be used.
func AssertProposalOnlyUse(usedAsEvidence, acceptedByUser bool, proposalRef string) []domain.Rejection {
	rejections := []domain.Rejection{}
	if usedAsEvidence {
		rejections = append(rejections, domain.NewRejection("INV.EVIDENCE.PROMOTED-CLAIM",
			fmt.Sprintf("proposal %s cannot be used as evidence: a model's output is a proposal until the user accepts it and independent evidence exists", proposalRef),
			"evidenceRefs"))
	}
	if !acceptedByUser {
		rejections = append(rejections, domain.NewRejection("MODEL.PROPOSAL.NOT-ACCEPTED",
			fmt.Sprintf("proposal %s has not been accepted by the user; it cannot influence a representation", proposalRef), "status"))
	}
	return rejections
}
