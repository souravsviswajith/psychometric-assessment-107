package engine

import (
	"fmt"
	"sort"

	"career107/domain"
)

// Capability → Authorization → S_1 → S_2 → S_3 → S_4.
//
// This is the only path by which work is executed. The runtime never calls a
// transformation directly: it plans, authorizes, materialises a platform
// representation, hands the work to the Cloud107 adapter, and records the
// execution with artifacts and digests.

type InvocationDeps struct {
	Registry      *CapabilityRegistry
	Authorization *AuthorizationService
	Cloud107      Cloud107Adapter
	Artifacts     *ArtifactStore
	Traces        *TraceLog
	Clock         domain.Clock
	Hasher        domain.Hasher
	RuntimeID     string
}

type TransformationDescriptor struct {
	TransformationID            string
	TransformationVersion       string
	InputRepresentationVersion  string
	OutputRepresentationVersion string
	Deterministic               bool
	RequiresModel               bool
	DeclaredLosses              []string
}

type InvokeParams struct {
	Request        domain.CapabilityRequest
	Payload        any
	Transformation TransformationDescriptor
	InputRefs      []string
	PolicyRefs     []string
	Constraints    []string
}

type InvocationOutcome struct {
	HandlerOutput CapabilityRunOutput
	Plan          ExecutionPlan
	Invocation    RuntimeInvocation
	Execution     ExecutionRecord
	Authorization AuthorizationDecision
	Value         any
	OutputRefs    []string
	ArtifactRefs  []string
	Trace         PipelineTrace
}

func InvokeThroughPipeline(deps InvocationDeps, params InvokeParams) (InvocationOutcome, []domain.Rejection) {
	request := params.Request
	at := deps.Clock.Now()
	trace := deps.Traces.Begin(request.CapabilityID, request.SubjectID, at)
	var rejections []domain.Rejection

	// ------------------------------------------------------ S1: validated representation
	requestIssues := domain.ValidateCapabilityRequest(request)
	rejections = append(rejections, domain.IssuesToRejections("S1.REQUEST.SHAPE", requestIssues, "request")...)
	handler, resolveRejections := deps.Registry.Resolve(request.CapabilityID)
	rejections = append(rejections, resolveRejections...)
	if len(params.InputRefs) == 0 {
		rejections = append(rejections, domain.NewRejection("S1.REQUEST.NO-INPUT-REFS", "capability invocation requires resolved input references", "inputRefs"))
	}
	if params.Payload == nil {
		rejections = append(rejections, domain.NewRejection("S1.REQUEST.NO-PAYLOAD", "capability invocation requires a validated payload", "payload"))
	}
	payloadBytes := 0
	if params.Payload != nil {
		payloadBytes = domain.UTF8ByteLength(domain.Canonicalize(params.Payload))
	}

	trace.RecordStage(RecordStageParams{
		Stage: "S1", Operation: request.CapabilityID, At: at,
		InputRefs: params.InputRefs, OutputRefs: []string{}, ChecksRun: 3,
		Valid: len(rejections) == 0, Rejections: toExecutionRejections(rejections),
	})
	if handler == nil || len(rejections) > 0 {
		trace.Finish(at)
		return InvocationOutcome{}, rejections
	}
	contract := handler.Contract()

	// ------------------------------------------------------ S2: constrained plan
	decision := deps.Authorization.Evaluate(EvaluateParams{Request: request, Contract: contract, InputBytes: payloadBytes, DecidedAt: at})
	if decision.Outcome == "denied" {
		denial := AuthorizationRejections(decision)
		trace.RecordStage(RecordStageParams{
			Stage: "S2", Operation: request.CapabilityID, At: at,
			InputRefs: params.InputRefs, OutputRefs: []string{decision.ID}, ChecksRun: 1,
			Valid: false, Rejections: toExecutionRejections(denial),
		})
		trace.Finish(at)
		return InvocationOutcome{}, denial
	}
	deps.Authorization.Consume(decision)

	// Career semantics must not cross the Cloud107 boundary (§8): refuse to plan
	// a payload that carries scoring rules or decisions.
	if leaked := ScanPayloadForCareerSemantics(params.Payload, ""); len(leaked) > 0 {
		boundary := make([]domain.Rejection, len(leaked))
		for i, finding := range leaked {
			boundary[i] = domain.NewRejection("S2.PLAN.SEMANTIC-BOUNDARY",
				fmt.Sprintf("payload field %s carries career semantics that must not cross the Cloud107 boundary", finding), finding)
		}
		trace.RecordStage(RecordStageParams{
			Stage: "S2", Operation: request.CapabilityID, At: at,
			InputRefs: params.InputRefs, OutputRefs: []string{decision.ID}, ChecksRun: 2,
			Valid: false, Rejections: toExecutionRejections(boundary),
		})
		trace.Finish(at)
		return InvocationOutcome{}, boundary
	}

	transformationRef := params.Transformation.TransformationID + "@" + params.Transformation.TransformationVersion
	sortedInputs := domain.CopyStrings(params.InputRefs)
	sort.Strings(sortedInputs)
	constraints := append([]string{
		"no network access is permitted during assessment execution",
		"input is data only: nothing from assessment input is executed",
	}, params.Constraints...)
	parameters := request.Parameters
	if parameters == nil {
		parameters = map[string]any{}
	}
	plan := ExecutionPlan{
		ID: domain.MintID("plan", map[string]any{
			"capabilityId":       request.CapabilityID,
			"subjectId":          request.SubjectID,
			"representationRefs": sortedInputs,
			"authorizationRef":   decision.ID,
			"transformation":     transformationRef,
		}),
		SchemaVersion:      RuntimeSchemaVersion,
		SubjectID:          request.SubjectID,
		CapabilityID:       request.CapabilityID,
		CapabilityVersion:  contract.Version,
		TransformationRef:  transformationRef,
		RepresentationRefs: domain.CopyStrings(params.InputRefs),
		SignalRefs:         []string{},
		PolicyRefs:         domain.CopyStrings(params.PolicyRefs),
		ParametersDigest:   domain.DigestOf(deps.Hasher, parameters, "plan:parameters"),
		MaxInputBytes:      contract.MaxInputBytes,
		EstimatedWork:      EstimatedWork{InputBytes: payloadBytes, RepresentationCount: len(params.InputRefs)},
		AuthorizationRef:   decision.ID,
		PlannedAt:          at,
		Constraints:        constraints,
	}
	if issues := ValidateExecutionPlan(plan); len(issues) > 0 {
		planRejections := domain.IssuesToRejections("S2.PLAN.SHAPE", issues, "plan")
		trace.RecordStage(RecordStageParams{
			Stage: "S2", Operation: request.CapabilityID, At: at,
			InputRefs: params.InputRefs, OutputRefs: []string{}, ChecksRun: 3,
			Valid: false, Rejections: toExecutionRejections(planRejections),
		})
		trace.Finish(at)
		return InvocationOutcome{}, planRejections
	}
	trace.RecordStage(RecordStageParams{
		Stage: "S2", Operation: request.CapabilityID, At: at,
		InputRefs: params.InputRefs, OutputRefs: []string{plan.ID, decision.ID}, ChecksRun: 3, Valid: true,
	})

	// ------------------------------------------------------ S3: platform representation
	payloadArtifact := deps.Artifacts.Put(PutArtifactParams{
		Value:     params.Payload,
		MediaType: "application/vnd.career107.capability-payload+json",
		CreatedAt: at,
		CreatedBy: deps.RuntimeID,
		SubjectID: plan.SubjectID,
	})
	invocation := RuntimeInvocation{
		ID: domain.MintID("plat", map[string]any{
			"planRef":       plan.ID,
			"payloadDigest": payloadArtifact.Digest.Value,
			"runtimeId":     deps.RuntimeID,
		}),
		SchemaVersion: RuntimeSchemaVersion,
		PlanRef:       plan.ID,
		CapabilityRef: contract.ID + "@" + contract.Version,
		PayloadRef:    payloadArtifact.ID,
		PayloadDigest: payloadArtifact.Digest,
		RequestedAt:   at,
		RuntimeID:     deps.RuntimeID,
		Limits:        InvocationLimits{MaxInputBytes: contract.MaxInputBytes, MaxWallClockMs: 15_000, AllowNetwork: false},
	}
	if issues := ValidateRuntimeInvocation(invocation); len(issues) > 0 {
		invocationRejections := domain.IssuesToRejections("S3.INVOCATION.SHAPE", issues, "invocation")
		trace.RecordStage(RecordStageParams{
			Stage: "S3", Operation: request.CapabilityID, At: at,
			InputRefs: []string{payloadArtifact.ID}, OutputRefs: []string{}, ChecksRun: 1,
			Valid: false, Rejections: toExecutionRejections(invocationRejections),
		})
		trace.Finish(at)
		return InvocationOutcome{}, invocationRejections
	}
	trace.RecordStage(RecordStageParams{
		Stage: "S3", Operation: request.CapabilityID, At: at,
		InputRefs: []string{payloadArtifact.ID}, OutputRefs: []string{invocation.ID, plan.ID}, ChecksRun: 2, Valid: true,
	})

	// ------------------------------------------------------ S4: execution + artifacts
	startedAt := deps.Clock.Now()
	executionID := domain.MintID("exe", map[string]any{
		"invocationRef": invocation.ID,
		"startedAt":     startedAt,
		"adapter":       deps.Cloud107.AdapterID(),
	})

	var handlerOutput *CapabilityRunOutput
	adapterResult := deps.Cloud107.Invoke(invocation, ExecutionContext{
		RuntimeID:  deps.RuntimeID,
		SubjectRef: plan.SubjectID,
		Parameters: parameters,
		Payload:    params.Payload,
		Execute: func() WorkOutcome {
			output, runRejections := handler.Run(CapabilityRunContext{RuntimeID: deps.RuntimeID, Plan: plan, Request: request, Payload: params.Payload})
			if len(runRejections) > 0 {
				return WorkOutcome{OK: false, OutputRefs: []string{}, Rejections: toExecutionRejections(runRejections)}
			}
			handlerOutput = &output
			return WorkOutcome{OK: true, Value: output.Value, OutputRefs: output.OutputRefs, Rejections: []ExecutionRejection{}}
		},
	})

	finishedAt := deps.Clock.Now()
	var resultArtifact *Artifact
	if adapterResult.Status == "succeeded" && adapterResult.HasResult {
		artifact := deps.Artifacts.Put(PutArtifactParams{
			Value:     adapterResult.Result,
			MediaType: "application/vnd.career107.capability-result+json",
			CreatedAt: finishedAt,
			CreatedBy: deps.Cloud107.AdapterID(),
			SubjectID: plan.SubjectID,
		})
		resultArtifact = &artifact
	}

	status := "failed"
	if adapterResult.Status == "succeeded" {
		status = "succeeded"
	}
	artifactRefs := domain.CopyStrings(adapterResult.ArtifactRefs)
	if resultArtifact != nil {
		artifactRefs = append(artifactRefs, resultArtifact.ID)
	}
	outputRefs := []string{}
	if adapterResult.Status == "succeeded" && handlerOutput != nil {
		outputRefs = domain.CopyStrings(handlerOutput.OutputRefs)
	}
	noteRejections := make([]ExecutionRejection, len(adapterResult.Notes))
	for i, note := range adapterResult.Notes {
		noteRejections[i] = ExecutionRejection{Code: "S4.EXECUTION.NOTE", Message: note}
	}
	infrastructure := map[string]string{}
	for key, value := range adapterResult.Infrastructure {
		infrastructure[key] = value
	}
	execution := ExecutionRecord{
		ID:            executionID,
		SchemaVersion: RuntimeSchemaVersion,
		InvocationRef: invocation.ID,
		PlanRef:       plan.ID,
		CapabilityRef: invocation.CapabilityRef,
		Status:        status,
		StartedAt:     startedAt,
		FinishedAt:    finishedAt,
		Executor: Executor{
			RuntimeID:      deps.RuntimeID,
			AdapterID:      deps.Cloud107.AdapterID(),
			Infrastructure: infrastructure,
		},
		ArtifactRefs:      artifactRefs,
		OutputRefs:        outputRefs,
		Rejections:        noteRejections,
		TransformationRef: transformationRef,
	}
	if issues := ValidateExecutionRecord(execution); len(issues) > 0 {
		return InvocationOutcome{}, domain.IssuesToRejections("S4.EXECUTION.SHAPE", issues, "execution")
	}

	artifactVerified := true
	if resultArtifact != nil {
		artifactVerified = deps.Artifacts.Verify(resultArtifact.ID).Verified
	}
	trace.RecordStage(RecordStageParams{
		Stage: "S4", Operation: request.CapabilityID, At: finishedAt,
		InputRefs:  []string{invocation.ID},
		OutputRefs: append([]string{execution.ID}, execution.ArtifactRefs...),
		ChecksRun:  3,
		Valid:      adapterResult.Status == "succeeded" && artifactVerified,
		Rejections: noteRejections,
	})
	finishedTrace := trace.Finish(finishedAt)

	if adapterResult.Status != "succeeded" {
		failed := make([]domain.Rejection, len(adapterResult.Notes))
		for i, note := range adapterResult.Notes {
			failed[i] = domain.NewRejection("S4.EXECUTION.FAILED", note, "execution")
		}
		return InvocationOutcome{}, failed
	}
	if !artifactVerified {
		ref := "<unknown>"
		if resultArtifact != nil {
			ref = resultArtifact.ID
		}
		return InvocationOutcome{}, []domain.Rejection{domain.NewRejection("S4.ARTIFACT.INTEGRITY", fmt.Sprintf("artifact %s failed digest verification", ref), "artifactRefs")}
	}

	output := CapabilityRunOutput{Value: adapterResult.Result, OutputRefs: []string{}}
	if handlerOutput != nil {
		output = *handlerOutput
	}
	return InvocationOutcome{
		HandlerOutput: output,
		Plan:          plan,
		Invocation:    invocation,
		Execution:     execution,
		Authorization: decision,
		Value:         adapterResult.Result,
		OutputRefs:    execution.OutputRefs,
		ArtifactRefs:  execution.ArtifactRefs,
		Trace:         finishedTrace,
	}, nil
}
