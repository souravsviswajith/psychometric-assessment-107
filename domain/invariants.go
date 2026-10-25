package domain

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// The §4 critical architectural invariant:
//
//	Never implement careerScore: number as the foundational representation.
//	Never collapse (A, E, C, U) into Score.
//
// This is enforced structurally: any representation is walked and scalar
// career-quantity fields are rejected.

// ForbiddenScoreKeys are field names that would collapse the representation
// into a scalar career quantity. Matching is case-insensitive on the
// normalised key.
var ForbiddenScoreKeys = []string{
	"careerscore",
	"careerpoints",
	"fitscore",
	"matchscore",
	"matchpercent",
	"overallscore",
	"compatibilityscore",
	"suitabilityscore",
	"aptitudescore",
	"potentialscore",
	"readinessscore",
	"weightedscore",
	"compositescore",
	"rank",
	"ranking",
	"bestcareer",
	"topcareer",
	"recommendedcareer",
	"careerrecommendation",
	"fitpercentage",
}

func ForbiddenScoreKeyReason(key string) string {
	normalized := normalizeKey(key)
	for _, forbidden := range ForbiddenScoreKeys {
		if normalized == forbidden || strings.Contains(normalized, forbidden) {
			return fmt.Sprintf("field %q collapses the (A, E, C, U) representation into a scalar career quantity", key)
		}
	}
	return ""
}

type ForbiddenKeyScanOptions struct {
	// Keys that are allowed even though they contain a forbidden substring.
	AllowKeys []string
	JSONPath  string
}

// ObjectKeys returns the keys of a generic JSON object in their serialised
// order, so structural walks report paths in a stable order.
func ObjectKeys(raw any) []string {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// ScanForScalarCareerQuantities is a deep scan for scalar career-quantity
// keys. It runs over any representation, record, or persistence shape (§4,
// §10), by walking exactly what would be serialised.
func ScanForScalarCareerQuantities(value any, options ForbiddenKeyScanOptions) []Rejection {
	allow := map[string]bool{}
	for _, key := range options.AllowKeys {
		allow[normalizeKey(key)] = true
	}
	rejections := []Rejection{}
	var walk func(node any, path string)
	walk = func(node any, path string) {
		switch v := node.(type) {
		case []any:
			for i, item := range v {
				walk(item, fmt.Sprintf("%s[%d]", path, i))
			}
		case map[string]any:
			for _, key := range ObjectKeys(v) {
				childPath := path + "." + key
				if !allow[normalizeKey(key)] {
					if reason := ForbiddenScoreKeyReason(key); reason != "" {
						rejections = append(rejections, NewRejection("INV.NO-SCALAR-CAREER-QUANTITY", reason, childPath))
					}
				}
				walk(v[key], childPath)
			}
		}
	}
	root := options.JSONPath
	if root == "" {
		root = "$"
	}
	walk(ToGeneric(value), root)
	return rejections
}

// NoScalarCollapseStatement states the architectural relationship:
//
//	1 observation ⇒ 3 representations ⇒ validated transformations
//	              ⇒ explainable pathway information
//
// — never 1 observation ⇒ 1 career conclusion.
const NoScalarCollapseStatement = "1 observation does not imply 1 career conclusion: representations are (A, E, C) with cross-cutting U."

// ------------------------------------------------------------- registry
//
// Named, executable invariants. Each invariant is a check with a stable
// identifier and a statement in reviewer language. They are executed by tests,
// by the runtime before transformation, and by the research experiments, so
// that "the invariant holds" is a measurement rather than a claim.

type InvariantCheck[T any] struct {
	ID        string
	Statement string
	Run       func(subject T) []Rejection
}

type InvariantDescriptor struct {
	ID        string `json:"id"`
	Statement string `json:"statement"`
	Scope     string `json:"scope"`
	Mandatory bool   `json:"mandatory"`
}

var InvariantDescriptors = []InvariantDescriptor{
	{ID: "INV.IDENTITY.CARRIED", Statement: "A.id = E.id = C.id = SignalId: identity is carried through the transformation, never re-minted.", Scope: "representation", Mandatory: true},
	{ID: "INV.EVIDENCE.PRIMARY", Statement: "Every observation has exactly one primary evidence record, and every derived record has lineage.", Scope: "signal", Mandatory: true},
	{ID: "INV.EVIDENCE.PROMOTED-CLAIM", Statement: "A promoted claim carries a non-null evidence reference; an unsupported claim is refused.", Scope: "signal", Mandatory: true},
	{ID: "INV.UNCERTAINTY.PRESERVED", Statement: "Uncertainty is preserved through transformations: output notes are a superset of input notes.", Scope: "transformation", Mandatory: true},
	{ID: "INV.NO-SCALAR-CAREER-QUANTITY", Statement: "No representation, record, or pathway collapses (A, E, C, U) into a scalar career quantity.", Scope: "representation", Mandatory: true},
	{ID: "INV.PATHWAY.SUPPORT", Statement: "Every pathway candidate references the representations and transformations that support it.", Scope: "pathway", Mandatory: true},
	{ID: "INV.PATHWAY.NO-RANK", Statement: `Pathway sets carry no total ordering, no hidden score, and no single "best career".`, Scope: "pathway", Mandatory: true},
	{ID: "INV.PATHWAY.USER-AUTHORITY", Statement: "Pathway review state is set by the user (0); the system records but never assigns a decision.", Scope: "pathway", Mandatory: true},
	{ID: "INV.VERSION.NON-DESTRUCTIVE", Statement: "Historical representations are versioned, never silently mutated.", Scope: "records", Mandatory: true},
	{ID: "INV.MODEL.AUDITABLE", Statement: "Model-dependent transformations record provider, model, model version, instruction version, input ref, output ref, uncertainty, and transformation version.", Scope: "transformation", Mandatory: true},
	{ID: "INV.INPUT.SIZE-LIMIT", Statement: "Assessment input is bounded and treated as data; no assessment input is executed.", Scope: "signal", Mandatory: true},
	{ID: "INV.UI.NOT-TRUTH", Statement: "The UI projects domain state; it does not define it.", Scope: "ui", Mandatory: false},
}

func DescriptorForInvariant(id string) (InvariantDescriptor, bool) {
	for _, d := range InvariantDescriptors {
		if d.ID == id {
			return d, true
		}
	}
	return InvariantDescriptor{}, false
}

var SignalInvariants = []InvariantCheck[Signal]{
	{
		ID:        "INV.IDENTITY.CARRIED",
		Statement: "primary evidence id and context id equal the signal id",
		Run: func(s Signal) []Rejection {
			var issues []Rejection
			primary, ok := PrimaryOf(s.Evidence)
			if !ok {
				issues = append(issues, NewRejection("INV.EVIDENCE.PRIMARY", "signal has no primary evidence record", "evidence"))
			} else {
				if primary.ID != s.ID {
					issues = append(issues, NewRejection("INV.IDENTITY.CARRIED", "primary evidence id differs from the signal id", "evidence[0].id"))
				}
				if primary.SignalID != s.ID {
					issues = append(issues, NewRejection("INV.IDENTITY.CARRIED", "primary evidence references a different signal", "evidence[0].signalId"))
				}
			}
			if s.Context.ID != s.ID {
				issues = append(issues, NewRejection("INV.IDENTITY.CARRIED", "context id differs from the signal id", "context.id"))
			}
			return issues
		},
	},
	{
		ID:        "INV.EVIDENCE.PRIMARY",
		Statement: "exactly one primary record; derived records carry lineage",
		Run: func(s Signal) []Rejection {
			var issues []Rejection
			primaries := 0
			for _, record := range s.Evidence {
				if record.Status == "primary" {
					primaries++
				}
			}
			if primaries != 1 {
				issues = append(issues, NewRejection("INV.EVIDENCE.PRIMARY", fmt.Sprintf("expected exactly one primary evidence record, found %d", primaries), "evidence"))
			}
			for i, record := range s.Evidence {
				if record.Status == "derived" && len(record.Provenance.Lineage) == 0 {
					issues = append(issues, NewRejection("INV.EVIDENCE.PRIMARY", "derived evidence has no lineage", fmt.Sprintf("evidence[%d].provenance.lineage", i)))
				}
			}
			return issues
		},
	},
	{
		ID:        "INV.EVIDENCE.PROMOTED-CLAIM",
		Statement: "promoted claims carry a non-null evidence reference",
		Run: func(s Signal) []Rejection {
			var issues []Rejection
			for i, record := range s.Evidence {
				if strings.TrimSpace(record.Provenance.SourceRef) == "" {
					issues = append(issues, NewRejection("INV.EVIDENCE.PROMOTED-CLAIM", "evidence has an empty source reference", fmt.Sprintf("evidence[%d]", i)))
				}
				if strings.TrimSpace(record.Method.Ref) == "" {
					issues = append(issues, NewRejection("INV.EVIDENCE.PROMOTED-CLAIM", "evidence has an empty method reference", fmt.Sprintf("evidence[%d]", i)))
				}
			}
			return issues
		},
	},
	{
		ID:        "INV.NO-SCALAR-CAREER-QUANTITY",
		Statement: "no scalar career quantity fields anywhere in the signal",
		Run: func(s Signal) []Rejection {
			return ScanForScalarCareerQuantities(s, ForbiddenKeyScanOptions{JSONPath: "signal"})
		},
	},
	{
		ID:        "INV.INPUT.SIZE-LIMIT",
		Statement: "response text stays inside the declared input limits",
		Run: func(s Signal) []Rejection {
			var issues []Rejection
			if s.Response.Kind == "text" && s.Response.CharCount > JSLength(s.Response.Text)+1 {
				issues = append(issues, NewRejection("INV.INPUT.SIZE-LIMIT", "recorded charCount exceeds the stored response", "response.charCount"))
			}
			if s.Response.Kind == "text" && JSLength(s.Response.Text) > 4000 {
				issues = append(issues, NewRejection("INV.INPUT.SIZE-LIMIT", "response text exceeds the input limit", "response.text"))
			}
			return issues
		},
	},
}

var RepresentationInvariants = []InvariantCheck[Representation1to3]{
	{
		ID:        "INV.IDENTITY.CARRIED",
		Statement: "A.id = E.id = C.id = signalId",
		Run: func(r Representation1to3) []Rejection {
			var issues []Rejection
			if r.A.ID != r.SignalID {
				issues = append(issues, NewRejection("INV.IDENTITY.CARRIED", "A.id does not equal the signal id", "A.id"))
			}
			if r.E.ID != r.SignalID {
				issues = append(issues, NewRejection("INV.IDENTITY.CARRIED", "E.id does not equal the signal id", "E.id"))
			}
			if r.C.ID != r.SignalID {
				issues = append(issues, NewRejection("INV.IDENTITY.CARRIED", "C.id does not equal the signal id", "C.id"))
			}
			if r.A.ID != r.E.ID || r.E.ID != r.C.ID {
				issues = append(issues, NewRejection("INV.IDENTITY.CARRIED", "A, E, and C do not share one identity", "A.id"))
			}
			return issues
		},
	},
	{
		ID:        "INV.EVIDENCE.PROMOTED-CLAIM",
		Statement: "the representation resolves its primary evidence reference",
		Run: func(r Representation1to3) []Rejection {
			primary, ok := r.PrimaryEvidence()
			if !ok {
				return []Rejection{NewRejection("INV.EVIDENCE.PROMOTED-CLAIM", "primary evidence reference does not resolve inside the bundle", "E.primaryEvidenceId")}
			}
			if strings.TrimSpace(primary.Provenance.SourceRef) == "" {
				return []Rejection{NewRejection("INV.EVIDENCE.PROMOTED-CLAIM", "primary evidence has no source reference", "E.bundle[0]")}
			}
			return nil
		},
	},
	{
		ID:        "INV.NO-SCALAR-CAREER-QUANTITY",
		Statement: "no scalar career quantity fields in the representation",
		Run: func(r Representation1to3) []Rejection {
			return ScanForScalarCareerQuantities(r, ForbiddenKeyScanOptions{JSONPath: "representation"})
		},
	},
	{
		ID:        "INV.UNCERTAINTY.PRESERVED",
		Statement: "uncertainty is represented explicitly, never absent-by-default",
		Run: func(r Representation1to3) []Rejection {
			var issues []Rejection
			if len(r.U.Notes) == 0 && r.U.Coverage == "assessed" {
				issues = append(issues, NewRejection("INV.UNCERTAINTY.PRESERVED", `coverage claims "assessed" with no notes recorded`, "U.notes"))
			}
			for i, note := range r.U.Notes {
				if LevelRank(note.Level) < LevelRank(LevelLow) {
					issues = append(issues, NewRejection("INV.UNCERTAINTY.PRESERVED", "uncertainty level is not representable", fmt.Sprintf("U.notes[%d].level", i)))
				}
			}
			return issues
		},
	},
}

var PathwayInvariants = []InvariantCheck[PathwaySet]{
	{
		ID:        "INV.PATHWAY.SUPPORT",
		Statement: "every pathway references supporting representations and transformations",
		Run: func(set PathwaySet) []Rejection {
			var issues []Rejection
			for i, pathway := range set.Pathways {
				if len(pathway.SupportingRepresentationRefs) == 0 {
					issues = append(issues, NewRejection("INV.PATHWAY.SUPPORT", fmt.Sprintf("pathway %s references no representation", pathway.ID), fmt.Sprintf("pathways[%d]", i)))
				}
				if len(pathway.SupportingTransformationRefs) == 0 {
					issues = append(issues, NewRejection("INV.PATHWAY.SUPPORT", fmt.Sprintf("pathway %s references no transformation", pathway.ID), fmt.Sprintf("pathways[%d]", i)))
				}
				for j, requirement := range pathway.Requirements {
					if len(requirement.EvidenceRefs) == 0 && requirement.Status != "undetermined" {
						issues = append(issues, NewRejection("INV.PATHWAY.SUPPORT",
							fmt.Sprintf("requirement %s of pathway %s claims a status without evidence", requirement.RequirementID, pathway.ID),
							fmt.Sprintf("pathways[%d].requirements[%d]", i, j)))
					}
				}
			}
			return issues
		},
	},
	{
		ID:        "INV.PATHWAY.NO-RANK",
		Statement: "no ordering, score, or single best career in the pathway set",
		Run: func(set PathwaySet) []Rejection {
			return ScanForScalarCareerQuantities(set, ForbiddenKeyScanOptions{JSONPath: "pathwaySet"})
		},
	},
	{
		ID:        "INV.PATHWAY.USER-AUTHORITY",
		Statement: "pathway review state is a user decision, marked as such",
		Run: func(set PathwaySet) []Rejection {
			var issues []Rejection
			for i, pathway := range set.Pathways {
				if pathway.Review.State != "unreviewed" && pathway.Review.DecidedBy.Kind != ActorUser {
					issues = append(issues, NewRejection("INV.PATHWAY.USER-AUTHORITY",
						fmt.Sprintf("pathway %s carries a review decision not made by the user", pathway.ID), fmt.Sprintf("pathways[%d].review", i)))
				}
			}
			return issues
		},
	},
}

var TransformationInvariants = []InvariantCheck[TransformationRecord]{
	{
		ID:        "INV.MODEL.AUDITABLE",
		Statement: "model-dependent transformations record full model metadata and uncertainty",
		Run: func(record TransformationRecord) []Rejection {
			var issues []Rejection
			if record.RequiresModel {
				if record.ModelMetadata == nil {
					return []Rejection{NewRejection("INV.MODEL.AUDITABLE", "model-dependent transformation has no model metadata", "modelMetadata")}
				}
				m := record.ModelMetadata
				required := [][2]string{
					{"provider", m.Provider},
					{"model", m.Model},
					{"modelVersion", m.ModelVersion},
					{"instructionVersion", m.InstructionVersion},
					{"inputRef", m.InputRef},
					{"outputRef", m.OutputRef},
				}
				for _, pair := range required {
					if strings.TrimSpace(pair[1]) == "" {
						issues = append(issues, NewRejection("INV.MODEL.AUDITABLE", fmt.Sprintf("model metadata is missing %q", pair[0]), "modelMetadata."+pair[0]))
					}
				}
				if strings.TrimSpace(m.Uncertainty) == "" {
					issues = append(issues, NewRejection("INV.MODEL.AUDITABLE", "model metadata records no uncertainty statement", "modelMetadata.uncertainty"))
				}
			}
			if strings.TrimSpace(record.TransformationVersion) == "" {
				issues = append(issues, NewRejection("INV.MODEL.AUDITABLE", "transformation version is empty", "transformationVersion"))
			}
			return issues
		},
	},
	{
		ID:        "INV.UNCERTAINTY.PRESERVED",
		Statement: "declared losses are recorded rather than silent",
		Run: func(record TransformationRecord) []Rejection {
			if len(record.DeclaredLosses) > 0 && len(record.UncertaintyRefs) == 0 {
				return []Rejection{NewRejection("INV.UNCERTAINTY.PRESERVED", "transformation declares information loss but records no uncertainty note for it", "uncertaintyRefs")}
			}
			return nil
		},
	},
}

var RecordInvariants = []InvariantCheck[AssessmentRecord]{
	{
		ID:        "INV.VERSION.NON-DESTRUCTIVE",
		Statement: "assessment records are append-only and carry a digest of content",
		Run: func(record AssessmentRecord) []Rejection {
			var issues []Rejection
			if strings.TrimSpace(record.ContentDigest.Value) == "" {
				issues = append(issues, NewRejection("INV.VERSION.NON-DESTRUCTIVE", "record has no content digest", "contentDigest"))
			}
			if record.Revision < 1 {
				issues = append(issues, NewRejection("INV.VERSION.NON-DESTRUCTIVE", "record revision must be >= 1", "revision"))
			}
			return issues
		},
	},
	{
		ID:        "INV.NO-SCALAR-CAREER-QUANTITY",
		Statement: "no scalar career quantity in assessment records",
		Run: func(record AssessmentRecord) []Rejection {
			return ScanForScalarCareerQuantities(record, ForbiddenKeyScanOptions{JSONPath: "assessmentRecord"})
		},
	},
}

func CheckInvariants[T any](checks []InvariantCheck[T], subject T) ValidationReport {
	rejections := []Rejection{}
	for _, check := range checks {
		rejections = append(rejections, check.Run(subject)...)
	}
	return Report(len(checks), rejections)
}

func CheckSignal(s Signal) ValidationReport { return CheckInvariants(SignalInvariants, s) }

func CheckRepresentation(r Representation1to3) ValidationReport {
	return CheckInvariants(RepresentationInvariants, r)
}

func CheckPathwaySet(set PathwaySet) ValidationReport { return CheckInvariants(PathwayInvariants, set) }

func CheckTransformationRecord(record TransformationRecord) ValidationReport {
	return CheckInvariants(TransformationInvariants, record)
}

func CheckAssessmentRecord(record AssessmentRecord) ValidationReport {
	return CheckInvariants(RecordInvariants, record)
}

// RawJSON is a helper for scanning arbitrary JSON documents (for example a
// persistence payload) with ScanForScalarCareerQuantities.
func RawJSON(data []byte) any {
	var out any
	_ = json.Unmarshal(data, &out)
	return out
}
