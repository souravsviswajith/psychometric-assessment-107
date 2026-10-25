package engine

import (
	"fmt"
	"strconv"

	"career107/domain"
)

// The registered capability handlers.
//
// Every handler is a thin adapter over a domain transformation: it validates
// its payload again at the boundary (defence in depth, §17), re-runs the
// domain invariants, and returns references. It never returns a career
// conclusion, and it never returns a scalar career quantity.

type HandlerDeps struct {
	Clock domain.Clock
	Model ModelProvider
}

type MapPathwaysPayload struct {
	Representations []domain.Representation1to3 `json:"representations"`
	// Review states carried forward from a previous set; the user's decisions
	// survive re-mapping.
	ExistingReviewStates map[string]string `json:"existingReviewStates,omitempty"`
}

type ProposalPayload struct {
	Representation     domain.Representation1to3 `json:"representation"`
	InstructionRef     string                    `json:"instructionRef"`
	InstructionVersion string                    `json:"instructionVersion"`
	InstructionText    string                    `json:"instructionText"`
}

// ProposalOutput is the value returned by the propose-questions handler.
type ProposalOutput struct {
	ProposalText  string                  `json:"proposalText"`
	ModelMetadata domain.ProviderMetadata `json:"modelMetadata"`
	OutputRef     string                  `json:"outputRef"`
	Findings      []Finding               `json:"findings"`
	Removals      []string                `json:"removals"`
}

type Finding struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func mustContract(id string) domain.CapabilityContract {
	contract, ok := domain.LookupCapabilityContract(id)
	if !ok {
		panic("undeclared capability " + id)
	}
	return contract
}

func payloadShape(message, path string) []domain.Rejection {
	return []domain.Rejection{domain.NewRejection("CAPABILITY.PAYLOAD.SHAPE", message, path)}
}

// ------------------------------------------------------------ decompose

type decomposeHandler struct{}

func (decomposeHandler) Contract() domain.CapabilityContract {
	return mustContract(domain.CapabilityDecompose1to3)
}

func (decomposeHandler) Run(ctx CapabilityRunContext) (CapabilityRunOutput, []domain.Rejection) {
	signal, ok := ctx.Payload.(domain.Signal)
	if !ok {
		return CapabilityRunOutput{}, payloadShape(fmt.Sprintf("S0 signal payload rejected: expected a signal, received %T", ctx.Payload), "signal")
	}
	if issues := domain.ValidateSignal(signal); len(issues) > 0 {
		rejections := make([]domain.Rejection, len(issues))
		for i, issue := range issues {
			path := issue.Path
			if path == "" {
				path = "signal"
			}
			rejections[i] = domain.NewRejection("CAPABILITY.PAYLOAD.SHAPE", "S0 signal payload rejected: "+issue.Message, path)
		}
		return CapabilityRunOutput{}, rejections
	}
	if report := domain.CheckSignal(signal); !report.Valid {
		return CapabilityRunOutput{}, report.Rejections
	}
	representation, rejections := domain.DecomposeSignal(domain.DecomposeParams{
		Signal:    signal,
		CreatedAt: ctx.Plan.PlannedAt,
		CreatedBy: domain.SystemActor,
	})
	if len(rejections) > 0 {
		return CapabilityRunOutput{}, rejections
	}
	if report := domain.CheckRepresentation(representation); !report.Valid {
		return CapabilityRunOutput{}, report.Rejections
	}
	evidenceRefs := make([]string, len(representation.E.Bundle))
	for i, record := range representation.E.Bundle {
		evidenceRefs[i] = record.ID
	}
	uncertaintyRefs := make([]string, len(representation.U.Notes))
	for i, note := range representation.U.Notes {
		uncertaintyRefs[i] = note.ID
	}
	return CapabilityRunOutput{
		Value:           representation,
		OutputRefs:      []string{representation.ID},
		EvidenceRefs:    evidenceRefs,
		UncertaintyRefs: uncertaintyRefs,
		DeclaredLosses: []string{
			"the raw response is kept verbatim, but downstream readings use the normalised position and the recorded interpretation policy",
			"the instrument prompt is retained as text; the instrument’s wider construct coverage is not carried into the representation",
		},
	}, nil
}

// --------------------------------------------------------- map pathways

type mapPathwaysHandler struct{}

func (mapPathwaysHandler) Contract() domain.CapabilityContract {
	return mustContract(domain.CapabilityMapPathways)
}

func (mapPathwaysHandler) Run(ctx CapabilityRunContext) (CapabilityRunOutput, []domain.Rejection) {
	payload, ok := ctx.Payload.(MapPathwaysPayload)
	if !ok || len(payload.Representations) == 0 {
		return CapabilityRunOutput{}, payloadShape("map-pathways requires an array of S1 representations", "representations")
	}
	var rejections []domain.Rejection
	representations := []domain.Representation1to3{}
	for i, candidate := range payload.Representations {
		prefix := "representations[" + strconv.Itoa(i) + "]."
		if issues := domain.ValidateRepresentation(candidate); len(issues) > 0 {
			for _, issue := range issues {
				rejections = append(rejections, domain.NewRejection("CAPABILITY.PAYLOAD.SHAPE", issue.Message, prefix+issue.Path))
			}
			continue
		}
		if report := domain.CheckRepresentation(candidate); !report.Valid {
			for _, item := range report.Rejections {
				rejections = append(rejections, domain.NewRejection(item.Code, item.Message, prefix+item.Path))
			}
			continue
		}
		representations = append(representations, candidate)
	}
	if len(rejections) > 0 {
		return CapabilityRunOutput{}, rejections
	}

	var existing map[string]domain.PathwayReview
	if payload.ExistingReviewStates != nil {
		existing = map[string]domain.PathwayReview{}
		for pathwayID, state := range payload.ExistingReviewStates {
			existing[pathwayID] = domain.PathwayReview{State: state, DecidedBy: domain.UserZeroActor}
		}
	}
	set, mapRejections := domain.MapToPathways(domain.MapPathwaysParams{
		SubjectID:       representations[0].SubjectID,
		Representations: representations,
		CreatedAt:       ctx.Plan.PlannedAt,
		CreatedBy:       domain.SystemActor,
		ExistingReviews: existing,
	})
	if len(mapRejections) > 0 {
		return CapabilityRunOutput{}, mapRejections
	}

	outputRefs := []string{set.ID}
	evidenceRefs := []string{}
	uncertaintyRefs := []string{}
	for _, pathway := range set.Pathways {
		outputRefs = append(outputRefs, pathway.ID)
		evidenceRefs = append(evidenceRefs, pathway.EvidenceRefs...)
		for _, note := range pathway.Uncertainty {
			uncertaintyRefs = append(uncertaintyRefs, note.ID)
		}
	}
	return CapabilityRunOutput{
		Value:           set,
		OutputRefs:      outputRefs,
		EvidenceRefs:    domain.UniqueStrings(evidenceRefs),
		UncertaintyRefs: domain.UniqueStrings(uncertaintyRefs),
		DeclaredLosses: []string{
			"pathway mapping " + domain.MapPathwaysTransformationVersion + " applies criterion bands over single observations",
			"pathway templates carry no labour-market data",
		},
	}, nil
}

// ---------------------------------------------------- propose questions

type proposeQuestionsHandler struct{ deps HandlerDeps }

func (proposeQuestionsHandler) Contract() domain.CapabilityContract {
	return mustContract(domain.CapabilityProposeQuestions)
}

func (h proposeQuestionsHandler) Run(ctx CapabilityRunContext) (CapabilityRunOutput, []domain.Rejection) {
	payload, ok := ctx.Payload.(ProposalPayload)
	if !ok {
		return CapabilityRunOutput{}, payloadShape("propose-questions requires a representation", "representation")
	}
	if issues := domain.ValidateRepresentation(payload.Representation); len(issues) > 0 {
		return CapabilityRunOutput{}, payloadShape("representation payload rejected: "+issues[0].Message, "representation")
	}
	if h.deps.Model == nil {
		return CapabilityRunOutput{}, []domain.Rejection{domain.NewRejection("CAPABILITY.MODEL-UNAVAILABLE",
			"propose-questions is model-dependent and no model provider is registered in this deployment", "model")}
	}
	instructionRef := payload.InstructionRef
	if instructionRef == "" {
		instructionRef = "instruction.propose-questions.v1"
	}
	instructionVersion := payload.InstructionVersion
	if instructionVersion == "" {
		instructionVersion = "1.0.0"
	}
	instructionText := payload.InstructionText
	if instructionText == "" {
		instructionText = "Propose up to three follow-up questions that would reduce uncertainty."
	}
	generated, rejections := h.deps.Model.Generate(ModelGenerationRequest{
		InstructionRef:     instructionRef,
		InstructionVersion: instructionVersion,
		InstructionText:    instructionText,
		InputRef:           payload.Representation.ID,
		SubjectID:          payload.Representation.SubjectID,
		MaxOutputChars:     4000,
	})
	if len(rejections) > 0 {
		return CapabilityRunOutput{}, rejections
	}
	assessment := AssessModelOutput(generated.Text, MaxProposalChars)
	metadata := ProviderMetadataFrom(generated, nil)
	findings := make([]Finding, len(assessment.Findings))
	for i, finding := range assessment.Findings {
		findings[i] = Finding{Code: finding.Code, Message: finding.Message}
	}
	return CapabilityRunOutput{
		Value: ProposalOutput{
			ProposalText:  assessment.Sanitised,
			ModelMetadata: metadata,
			OutputRef:     generated.OutputRef,
			Findings:      findings,
			Removals:      assessment.Removals,
		},
		OutputRefs:      []string{generated.OutputRef, payload.Representation.ID},
		EvidenceRefs:    []string{},
		UncertaintyRefs: []string{},
		DeclaredLosses:  []string{"model-generated text is an interpretation of the representation, not an observation"},
	}, nil
}

// CreateDomainHandlers returns the handlers for every implemented capability.
func CreateDomainHandlers(deps HandlerDeps) []CapabilityHandler {
	return []CapabilityHandler{decomposeHandler{}, mapPathwaysHandler{}, proposeQuestionsHandler{deps: deps}}
}
