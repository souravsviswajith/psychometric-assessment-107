package domain

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

// The transformation contract.
//
// A transformation is the only sanctioned way to derive information from
// representations (S_1 → S_2 → S_4). It declares its inputs, outputs,
// version, information losses, and — when it depends on a model — the full
// model metadata required by §5.
//
// Invariant: the output may expand information; it may not silently delete it.

// ProviderMetadata is §5 model metadata, required for every model-dependent
// operation: provider, model, version, instruction version, input ref, output
// ref, uncertainty, and the transformation version.
type ProviderMetadata struct {
	Provider           string `json:"provider"`
	Model              string `json:"model"`
	ModelVersion       string `json:"modelVersion"`
	InstructionVersion string `json:"instructionVersion"`
	InputRef           string `json:"inputRef"`
	OutputRef          string `json:"outputRef"`
	Uncertainty        string `json:"uncertainty"`
	GeneratedAt        string `json:"generatedAt"`
	Determinism        string `json:"determinism"`
	// Prompt/instruction text digest — the instruction that produced the output.
	InstructionDigest *Digest  `json:"instructionDigest,omitempty"`
	Temperature       *float64 `json:"temperature,omitempty"`
}

var digestValuePattern = regexp.MustCompile(`^[0-9a-f]{16,128}$`)

func checkDigest(c *checker, path string, d Digest) {
	c.enum(joinPath(path, "algorithm"), d.Algorithm, DigestAlgorithms)
	c.enum(joinPath(path, "strength"), d.Strength, DigestStrengths)
	c.match(joinPath(path, "value"), d.Value, digestValuePattern, "digest value must be lowercase hex")
	c.str(joinPath(path, "covers"), d.Covers, 1, -1)
}

// ValidateDigest checks a digest against its schema.
func ValidateDigest(d Digest) []Issue {
	c := &checker{}
	checkDigest(c, "", d)
	return c.result()
}

func checkProviderMetadata(c *checker, path string, m ProviderMetadata) {
	c.str(joinPath(path, "provider"), m.Provider, 1, 80)
	c.str(joinPath(path, "model"), m.Model, 1, 120)
	c.str(joinPath(path, "modelVersion"), m.ModelVersion, 1, 80)
	c.str(joinPath(path, "instructionVersion"), m.InstructionVersion, 1, 120)
	c.str(joinPath(path, "inputRef"), m.InputRef, 1, 200)
	c.str(joinPath(path, "outputRef"), m.OutputRef, 1, 200)
	c.str(joinPath(path, "uncertainty"), m.Uncertainty, 1, 500)
	checkIso(c, joinPath(path, "generatedAt"), m.GeneratedAt)
	c.enum(joinPath(path, "determinism"), m.Determinism, []string{"deterministic", "sampled"})
	if m.InstructionDigest != nil {
		checkDigest(c, joinPath(path, "instructionDigest"), *m.InstructionDigest)
	}
	if m.Temperature != nil && (*m.Temperature < 0 || *m.Temperature > 2 || math.IsNaN(*m.Temperature)) {
		c.add(joinPath(path, "temperature"), "temperature must be within [0, 2]")
	}
}

func ValidateProviderMetadata(m ProviderMetadata) []Issue {
	c := &checker{}
	checkProviderMetadata(c, "", m)
	return c.result()
}

type TransformationRecord struct {
	ID                          string            `json:"id"`
	TransformationID            string            `json:"transformationId"`
	TransformationVersion       string            `json:"transformationVersion"`
	InputRepresentationVersion  string            `json:"inputRepresentationVersion"`
	OutputRepresentationVersion string            `json:"outputRepresentationVersion"`
	ExecutedAt                  string            `json:"executedAt"`
	ExecutedBy                  ActorRef          `json:"executedBy"`
	Deterministic               bool              `json:"deterministic"`
	RequiresModel               bool              `json:"requiresModel"`
	ModelMetadata               *ProviderMetadata `json:"modelMetadata,omitempty"`
	InputRefs                   []string          `json:"inputRefs"`
	OutputRefs                  []string          `json:"outputRefs"`
	EvidenceRefs                []string          `json:"evidenceRefs"`
	UncertaintyRefs             []string          `json:"uncertaintyRefs"`
	DeclaredLosses              []string          `json:"declaredLosses"`
	CapabilityRef               string            `json:"capabilityRef,omitempty"`
	AuthorizationRef            string            `json:"authorizationRef,omitempty"`
}

func ValidateTransformationRecord(r TransformationRecord) []Issue {
	c := &checker{}
	TransformationIDKind.check(c, "id", r.ID)
	c.str("transformationId", r.TransformationID, 1, 120)
	checkSemver(c, "transformationVersion", r.TransformationVersion)
	checkSemver(c, "inputRepresentationVersion", r.InputRepresentationVersion)
	checkSemver(c, "outputRepresentationVersion", r.OutputRepresentationVersion)
	checkIso(c, "executedAt", r.ExecutedAt)
	checkActor(c, "executedBy", r.ExecutedBy)
	if r.ModelMetadata != nil {
		checkProviderMetadata(c, "modelMetadata", *r.ModelMetadata)
	}
	c.strings("inputRefs", r.InputRefs, 1, -1, 1, 64)
	c.strings("outputRefs", r.OutputRefs, 1, -1, 1, 64)
	c.strings("evidenceRefs", r.EvidenceRefs, 1, -1, 0, 128)
	c.strings("uncertaintyRefs", r.UncertaintyRefs, 1, -1, 0, 64)
	c.strings("declaredLosses", r.DeclaredLosses, 1, 300, 0, 12)
	c.optStr("capabilityRef", r.CapabilityRef, 1, 120)
	c.optStr("authorizationRef", r.AuthorizationRef, 1, 120)
	return c.result()
}

// TransformationSpec is what a transformation declares about itself, before
// it runs.
type TransformationSpec struct {
	TransformationID            string
	TransformationVersion       string
	Name                        string
	Description                 string
	InputRepresentationVersion  string
	OutputRepresentationVersion string
	Deterministic               bool
	RequiresModel               bool
	// Information this transformation is known to discard or abstract (§11).
	DeclaredLosses []string
	CapabilityRef  string
}

type TransformationOutcome[Out any] struct {
	Value  Out
	Record TransformationRecord
}

func TransformationLabel(spec TransformationSpec) string {
	return spec.TransformationID + "@" + spec.TransformationVersion
}

var transformationRefPattern = regexp.MustCompile(`^(.+)@(\d+\.\d+\.\d+)$`)

func ParseTransformationRef(ref string) (id, version string, ok bool) {
	match := transformationRefPattern.FindStringSubmatch(ref)
	if match == nil {
		return "", "", false
	}
	return match[1], match[2], true
}

type RecordTransformationParams[Out any] struct {
	Spec             TransformationSpec
	Output           Out
	InputRefs        []string
	OutputRefs       []string
	EvidenceRefs     []string
	UncertaintyRefs  []string
	DeclaredLosses   []string
	HasLosses        bool // DeclaredLosses was supplied (otherwise the spec's losses apply)
	ExecutedAt       string
	ExecutedBy       ActorRef
	ModelMetadata    *ProviderMetadata
	CapabilityRef    string
	AuthorizationRef string
}

// RecordTransformation records a transformation execution, enforcing §5 model
// metadata and §11 loss declarations before the outcome is allowed to exist.
func RecordTransformation[Out any](params RecordTransformationParams[Out]) (TransformationOutcome[Out], []Rejection) {
	var rejections []Rejection
	if len(params.InputRefs) == 0 {
		rejections = append(rejections, NewRejection("S2.TRANSFORMATION.NO-INPUT-REF", "transformation input is not referenced", "inputRefs"))
	}
	if len(params.OutputRefs) == 0 {
		rejections = append(rejections, NewRejection("S2.TRANSFORMATION.NO-OUTPUT-REF", "transformation output is not referenced", "outputRefs"))
	}
	if params.Spec.RequiresModel && params.ModelMetadata == nil {
		rejections = append(rejections, NewRejection("INV.MODEL.AUDITABLE",
			fmt.Sprintf("transformation %s requires a model, so provider/model/version/instruction/input/output metadata is mandatory", params.Spec.TransformationID),
			"modelMetadata"))
	}
	if !params.Spec.RequiresModel && params.ModelMetadata != nil {
		rejections = append(rejections, NewRejection("S2.TRANSFORMATION.UNEXPECTED-MODEL",
			fmt.Sprintf("transformation %s declares itself model-independent but carries model metadata", params.Spec.TransformationID),
			"modelMetadata"))
	}
	losses := CopyStrings(params.Spec.DeclaredLosses)
	if params.HasLosses {
		losses = CopyStrings(params.DeclaredLosses)
	}
	if len(losses) > 0 && len(params.UncertaintyRefs) == 0 {
		rejections = append(rejections, NewRejection("INV.UNCERTAINTY.PRESERVED",
			"transformation declares information loss, so it must record an uncertainty note for that loss", "uncertaintyRefs"))
	}
	if params.Spec.Deterministic && params.Spec.RequiresModel {
		rejections = append(rejections, NewRejection("S2.TRANSFORMATION.DETERMINISM-CLAIM",
			"a model-dependent transformation cannot claim bit-level determinism", "deterministic"))
	}
	if len(rejections) > 0 {
		return TransformationOutcome[Out]{}, rejections
	}

	sortedInputs := CopyStrings(params.InputRefs)
	sort.Strings(sortedInputs)
	sortedOutputs := CopyStrings(params.OutputRefs)
	sort.Strings(sortedOutputs)
	candidate := TransformationRecord{
		ID: MintID("trf", map[string]any{
			"transformationId":      params.Spec.TransformationID,
			"transformationVersion": params.Spec.TransformationVersion,
			"inputRefs":             sortedInputs,
			"outputRefs":            sortedOutputs,
			"executedAt":            params.ExecutedAt,
		}),
		TransformationID:            params.Spec.TransformationID,
		TransformationVersion:       params.Spec.TransformationVersion,
		InputRepresentationVersion:  params.Spec.InputRepresentationVersion,
		OutputRepresentationVersion: params.Spec.OutputRepresentationVersion,
		ExecutedAt:                  params.ExecutedAt,
		ExecutedBy:                  params.ExecutedBy,
		Deterministic:               params.Spec.Deterministic,
		RequiresModel:               params.Spec.RequiresModel,
		ModelMetadata:               params.ModelMetadata,
		InputRefs:                   CopyStrings(params.InputRefs),
		OutputRefs:                  CopyStrings(params.OutputRefs),
		EvidenceRefs:                CopyStrings(params.EvidenceRefs),
		UncertaintyRefs:             CopyStrings(params.UncertaintyRefs),
		DeclaredLosses:              losses,
		CapabilityRef:               params.CapabilityRef,
		AuthorizationRef:            params.AuthorizationRef,
	}

	if issues := ValidateTransformationRecord(candidate); len(issues) > 0 {
		return TransformationOutcome[Out]{}, IssuesToRejections("S2.TRANSFORMATION.SHAPE", issues, "transformationRecord")
	}
	if report := CheckTransformationRecord(candidate); !report.Valid {
		return TransformationOutcome[Out]{}, report.Rejections
	}
	return TransformationOutcome[Out]{Value: params.Output, Record: candidate}, nil
}

// ----------------------------------------------------------------- versions
//
// The version set a result must be reproducible from (§11): input +
// representation version + transformation version + evidence references +
// model metadata (where applicable).

type VersionSet struct {
	RepresentationVersion     string            `json:"representationVersion"`
	TransformationVersion     string            `json:"transformationVersion"`
	EvidenceRefs              []string          `json:"evidenceRefs"`
	ModelMetadata             *ProviderMetadata `json:"modelMetadata,omitempty"`
	InstrumentVersion         string            `json:"instrumentVersion,omitempty"`
	UncertaintyProfileVersion string            `json:"uncertaintyProfileVersion,omitempty"`
	ContextSchemaVersion      string            `json:"contextSchemaVersion,omitempty"`
}

type ReproducibilityCheck struct {
	Reproducible  bool     `json:"reproducible"`
	MissingInputs []string `json:"missingInputs"`
}

func CheckReproducibility(versionSet VersionSet, modelDependent bool) ReproducibilityCheck {
	missing := []string{}
	if modelDependent && versionSet.ModelMetadata == nil {
		missing = append(missing, "modelMetadata (required for model-dependent operations)")
	}
	if len(versionSet.EvidenceRefs) == 0 {
		missing = append(missing, "evidenceRefs")
	}
	if strings.TrimSpace(versionSet.TransformationVersion) == "" {
		missing = append(missing, "transformationVersion")
	}
	if strings.TrimSpace(versionSet.RepresentationVersion) == "" {
		missing = append(missing, "representationVersion")
	}
	return ReproducibilityCheck{Reproducible: len(missing) == 0, MissingInputs: missing}
}

func RejectNonReproducible(versionSet VersionSet, modelDependent bool) []Rejection {
	check := CheckReproducibility(versionSet, modelDependent)
	out := []Rejection{}
	for _, input := range check.MissingInputs {
		out = append(out, NewRejection("INV.VERSION.REPRODUCIBILITY", "result is not reproducible: missing "+input, "versionSet"))
	}
	return out
}

// ContractVersionEntry is one named contract version, kept in display order.
type ContractVersionEntry struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ContractVersions are the current versions of the 107 semantic contracts,
// for display and auditing.
var ContractVersions = []ContractVersionEntry{
	{"signalSchema", "1.0.0"},
	{"representationSchema", "1.3.0"},
	{"assessmentFacet", "1.0.0"},
	{"evidenceSchema", "1.0.0"},
	{"contextSchema", "1.0.0"},
	{"uncertaintyProfile", "1.0.0"},
	{"pathwaySetSchema", "1.0.0"},
	{"recordsSchema", "1.0.0"},
	{"capabilityContracts", "0.1.0"},
	{"runtimeSchema", "0.1.0"},
}

func ContractVersion(name string) string {
	for _, entry := range ContractVersions {
		if entry.Name == name {
			return entry.Version
		}
	}
	return ""
}
