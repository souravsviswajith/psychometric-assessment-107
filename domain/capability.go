package domain

import (
	"fmt"
	"math"
)

// The domain side of the capability boundary (§9).
//
// A capability declares: what it is for, which transformation it applies,
// which scopes an invocation requires, whether it depends on a model, and what
// its input and output representations are. The runtime implements registry,
// authorization, and invocation; the domain owns these contracts so that
// career semantics cannot drift into infrastructure code.
//
//	Capability → Authorization → S_1 → S_2 → Execution → S_4

const CapabilityContractVersion = "0.1.0"

var CapabilityScopes = []string{
	"signal.capture",
	"representation.decompose",
	"representation.explain",
	"pathway.map",
	"proposal.generate",
	"records.write",
	"research.execute",
}

const (
	CapabilityDecompose1to3         = "career.assessment.decompose-1to3"
	CapabilityMapPathways           = "career.assessment.map-pathways"
	CapabilityProposeQuestions      = "career.assessment.propose-questions"
	CapabilityExplainRepresentation = "career.assessment.explain-representation"
)

type CapabilityContract struct {
	ID          string `json:"id"`
	Version     string `json:"version"`
	Description string `json:"description"`
	// Scopes the invocation must hold.
	RequiredScopes []string `json:"requiredScopes"`
	// True when the capability calls a language model (§5 metadata becomes mandatory).
	RequiresModel               bool   `json:"requiresModel"`
	TransformationRef           string `json:"transformationRef,omitempty"`
	InputRepresentationVersion  string `json:"inputRepresentationVersion"`
	OutputRepresentationVersion string `json:"outputRepresentationVersion"`
	// Maximum canonical input size in bytes (§17).
	MaxInputBytes int `json:"maxInputBytes"`
	// What this capability is explicitly not permitted to do at the 0 boundary.
	UserBoundary    string `json:"userBoundary"`
	InputSchemaRef  string `json:"inputSchemaRef"`
	OutputSchemaRef string `json:"outputSchemaRef"`
}

const s0Version = "1.0.0"

// CapabilityContracts lists every declared capability, in declaration order.
var CapabilityContracts = []CapabilityContract{
	{
		ID:                          CapabilityDecompose1to3,
		Version:                     CapabilityContractVersion,
		Description:                 "Decompose a captured signal into its (A, E, C) representation with cross-cutting uncertainty. Deterministic; the model is not involved.",
		RequiredScopes:              []string{"representation.decompose", "records.write"},
		RequiresModel:               false,
		TransformationRef:           DecomposeTransformationID + "@" + DecomposeTransformationVersion,
		InputRepresentationVersion:  s0Version,
		OutputRepresentationVersion: "1.3.0",
		MaxInputBytes:               32_768,
		UserBoundary:                "Decomposition never evaluates the observation: it produces (A, E, C) and uncertainty, and no conclusion about the person.",
		InputSchemaRef:              "career-domain:signal.types#Signal",
		OutputSchemaRef:             "career-domain:representation.types#Representation1to3",
	},
	{
		ID:                          CapabilityMapPathways,
		Version:                     CapabilityContractVersion,
		Description:                 "Map validated 1:3 representations onto criterion-referenced pathway information, preserving uncertainty and citing evidence.",
		RequiredScopes:              []string{"pathway.map", "records.write"},
		RequiresModel:               false,
		TransformationRef:           MapPathwaysTransformationID + "@" + MapPathwaysTransformationVersion,
		InputRepresentationVersion:  "1.3.0",
		OutputRepresentationVersion: "1.0.0",
		MaxInputBytes:               262_144,
		UserBoundary:                "Mapping produces alternative pathway information only. It never ranks, never selects, and never states a recommendation.",
		InputSchemaRef:              "career-domain:representation.types#Representation1to3[]",
		OutputSchemaRef:             "career-domain:pathway.types#PathwaySet",
	},
	{
		ID:                          CapabilityProposeQuestions,
		Version:                     CapabilityContractVersion,
		Description:                 "Propose candidate follow-up questions from gaps and uncertainties in the current representation. Model-dependent; output is a proposal, never an observation.",
		RequiredScopes:              []string{"proposal.generate"},
		RequiresModel:               true,
		InputRepresentationVersion:  "1.3.0",
		OutputRepresentationVersion: "1.0.0",
		MaxInputBytes:               65_536,
		UserBoundary:                "Proposed questions are shown to the user for review. They do not become assessments, they do not carry evidence, and the user decides whether to answer them.",
		InputSchemaRef:              "career-domain:representation.types#Representation1to3",
		OutputSchemaRef:             "career-domain:records.types#ProposalRecord",
	},
	{
		ID:                          CapabilityExplainRepresentation,
		Version:                     CapabilityContractVersion,
		Description:                 "Render a representation into reviewer-readable prose, with provenance and uncertainty attached. Model-dependent prose is marked as model-generated.",
		RequiredScopes:              []string{"representation.explain"},
		RequiresModel:               true,
		InputRepresentationVersion:  "1.3.0",
		OutputRepresentationVersion: "1.0.0",
		MaxInputBytes:               65_536,
		UserBoundary:                "Explanation restates the representation. It may not introduce new assessment claims or a career conclusion.",
		InputSchemaRef:              "career-domain:representation.types#Representation1to3",
		OutputSchemaRef:             "career-domain:records.types#ProposalRecord",
	},
}

// CapabilityAllowlist is the set of declared capability ids.
func CapabilityAllowlist() []string {
	ids := make([]string, len(CapabilityContracts))
	for i, contract := range CapabilityContracts {
		ids[i] = contract.ID
	}
	return ids
}

// ValidCapabilityID reports whether value is a dotted lowercase namespace.
func ValidCapabilityID(value string) bool {
	return capabilityIDPattern.MatchString(value)
}

func LookupCapabilityContract(capabilityID string) (CapabilityContract, bool) {
	for _, contract := range CapabilityContracts {
		if contract.ID == capabilityID {
			return contract, true
		}
	}
	return CapabilityContract{}, false
}

func IsAllowlistedCapability(capabilityID string) bool {
	_, ok := LookupCapabilityContract(capabilityID)
	return ok
}

// CapabilityRequest is the S_2 request shape. Parameters are bounded and
// declarative: a capability invocation cannot smuggle in executable content,
// and every reference must resolve to a stored representation (§9, §17).
type CapabilityRequest struct {
	CapabilityID      string   `json:"capabilityId"`
	CapabilityVersion string   `json:"capabilityVersion"`
	SubjectID         string   `json:"subjectId"`
	RequestedBy       ActorRef `json:"requestedBy"`
	SessionRef        string   `json:"sessionRef"`
	InputRefs         []string `json:"inputRefs"`
	// Parameter values are strings, finite numbers, or booleans.
	Parameters  map[string]any `json:"parameters"`
	RequestedAt string         `json:"requestedAt"`
	// Bind the invocation to a prior authorization decision, when one exists.
	AuthorizationRef string `json:"authorizationRef,omitempty"`
}

func ValidateCapabilityRequest(r CapabilityRequest) []Issue {
	c := &checker{}
	checkCapabilityID(c, "capabilityId", r.CapabilityID)
	checkSemver(c, "capabilityVersion", r.CapabilityVersion)
	SubjectIDKind.check(c, "subjectId", r.SubjectID)
	checkActor(c, "requestedBy", r.RequestedBy)
	c.str("sessionRef", r.SessionRef, 1, 120)
	c.strings("inputRefs", r.InputRefs, 1, -1, 1, 64)
	for key, value := range r.Parameters {
		path := joinPath("parameters", key)
		c.str(path, key, 1, 64)
		switch v := value.(type) {
		case string:
			c.str(path, v, 0, 400)
		case bool:
		case float64:
			if math.IsNaN(v) || math.IsInf(v, 0) {
				c.add(path, "Invalid input: expected number, received NaN")
			}
		case int:
		default:
			c.add(path, fmt.Sprintf("Invalid input: parameter values must be strings, numbers, or booleans, received %T", value))
		}
	}
	checkIso(c, "requestedAt", r.RequestedAt)
	c.optStr("authorizationRef", r.AuthorizationRef, 1, 120)
	return c.result()
}

const (
	DenialNotAllowlisted     = "not-allowlisted"
	DenialUnknownCapability  = "unknown-capability"
	DenialActorNotUserBound  = "actor-not-user-bound"
	DenialMissingScopes      = "missing-scopes"
	DenialExpiredGrant       = "expired-grant"
	DenialSubjectNotGranted  = "subject-not-granted"
	DenialBudgetExceeded     = "budget-exceeded"
	DenialInputTooLarge      = "input-too-large"
	DenialInvalidRequest     = "invalid-request"
	DenialNoAuthorization    = "no-authorization"
	DenialInvariantViolation = "invariant-violation"
)

type AuthorizationGrant struct {
	ID string `json:"id"`
	// Who the grant authorizes (user, capability, or system actor).
	Actor      ActorRef `json:"actor"`
	Scopes     []string `json:"scopes"`
	SubjectIDs []string `json:"subjectIds"`
	IssuedAt   string   `json:"issuedAt"`
	ExpiresAt  string   `json:"expiresAt,omitempty"`
	// Maximum number of invocations per capability id.
	Budget        map[string]int `json:"budget"`
	MaxInputBytes *int           `json:"maxInputBytes,omitempty"`
}
