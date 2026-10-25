package domain

import (
	"fmt"
	"sort"
	"strings"
)

// Pathway information (P).
//
//	M(A, E, C, U) → { P_1 … P_n }
//
// A pathway candidate is information: it states what a career direction would
// require, which observations bear on those requirements, what remains
// unknown, and what the trade-offs are. It never states that the user should
// choose it, and a pathway set is never ordered by a hidden score (§7, §18).

const PathwaySetSchemaVersion = "1.0.0"

var RequirementStatuses = []string{
	"supported",    // the referenced observations bear on the requirement and point the same way
	"conflicting",  // referenced observations disagree with each other
	"gap",          // referenced observations point away from the requirement
	"undetermined", // no observation bears on it, or uncertainty is too high to read it
}

type RequirementObservation struct {
	RepresentationRef string   `json:"representationRef"`
	SignalRef         string   `json:"signalRef"`
	Value             string   `json:"value"`
	EvidenceRefs      []string `json:"evidenceRefs"`
	UncertaintyRefs   []string `json:"uncertaintyRefs"`
}

type PathwayRequirement struct {
	RequirementID string `json:"requirementId"`
	// Construct the requirement is stated in terms of (project vocabulary).
	ConstructID string `json:"constructId"`
	Statement   string `json:"statement"`
	Kind        string `json:"kind"`
	Status      string `json:"status"`
	// Observations that bear on this requirement, with the values observed.
	Observations []RequirementObservation `json:"observations"`
	EvidenceRefs []string                 `json:"evidenceRefs"`
	// Why the status was assigned — in reviewer language, citing the policy.
	Rationale string `json:"rationale"`
	PolicyRef string `json:"policyRef"`
}

type Tradeoff struct {
	Statement string `json:"statement"`
	CostsWhat string `json:"costsWhat"`
	SourceRef string `json:"sourceRef"`
}

var PathwayReviewStates = []string{
	"unreviewed",
	"interesting", // user marked it worth exploring
	"not-a-fit",   // user rejected it
	"pursuing",    // user decided to act on it
	"deferred",
}

type PathwayReview struct {
	State     string   `json:"state"`
	DecidedBy ActorRef `json:"decidedBy"`
	DecidedAt string   `json:"decidedAt,omitempty"`
	Rationale string   `json:"rationale,omitempty"`
}

// SupportSummary is counters, not a score. It exists to be read, not to be
// compared as a ranking: two pathways with identical counters are not
// equivalent, and a higher counter is not a better direction.
type SupportSummary struct {
	Supported    int `json:"supported"`
	Conflicting  int `json:"conflicting"`
	Gap          int `json:"gap"`
	Undetermined int `json:"undetermined"`
	Total        int `json:"total"`
}

// PathwayCandidate. Note the absence of any numeric fit field: no fit score,
// no probability, no rank. Support is expressed through requirement statuses,
// counters, and citations.
type PathwayCandidate struct {
	ID                           string               `json:"id"`
	PathwayID                    string               `json:"pathwayId"`
	Name                         string               `json:"name"`
	Family                       string               `json:"family"`
	Description                  string               `json:"description"`
	Requirements                 []PathwayRequirement `json:"requirements"`
	SupportSummary               SupportSummary       `json:"supportSummary"`
	EntryRoutes                  []string             `json:"entryRoutes"`
	Tradeoffs                    []Tradeoff           `json:"tradeoffs"`
	OpenQuestions                []string             `json:"openQuestions"`
	Uncertainty                  []UncertaintyNote    `json:"uncertainty"`
	SupportingRepresentationRefs []string             `json:"supportingRepresentationRefs"`
	SupportingTransformationRefs []string             `json:"supportingTransformationRefs"`
	EvidenceRefs                 []string             `json:"evidenceRefs"`
	// Template the candidate was instantiated from, and its source reference.
	SourceRef string        `json:"sourceRef"`
	Review    PathwayReview `json:"review"`
}

func checkPathwayCandidate(c *checker, path string, p PathwayCandidate) {
	PathwayIDKind.check(c, joinPath(path, "id"), p.ID)
	c.str(joinPath(path, "pathwayId"), p.PathwayID, 1, 80)
	c.str(joinPath(path, "name"), p.Name, 1, 120)
	c.str(joinPath(path, "family"), p.Family, 1, 80)
	c.str(joinPath(path, "description"), p.Description, 1, 800)
	reqPath := joinPath(path, "requirements")
	c.arr(reqPath, len(p.Requirements), 1, 8)
	for i, r := range p.Requirements {
		rp := idx(reqPath, i)
		c.str(joinPath(rp, "requirementId"), r.RequirementID, 1, 80)
		c.str(joinPath(rp, "constructId"), r.ConstructID, 1, 80)
		c.str(joinPath(rp, "statement"), r.Statement, 1, 400)
		c.enum(joinPath(rp, "kind"), r.Kind, []string{"alignment", "constraint", "experience"})
		c.enum(joinPath(rp, "status"), r.Status, RequirementStatuses)
		for j, o := range r.Observations {
			op := idx(joinPath(rp, "observations"), j)
			c.str(joinPath(op, "representationRef"), o.RepresentationRef, 1, -1)
			c.str(joinPath(op, "signalRef"), o.SignalRef, 1, -1)
			c.str(joinPath(op, "value"), o.Value, 1, 200)
			c.strings(joinPath(op, "evidenceRefs"), o.EvidenceRefs, 1, -1, 1, -1)
			c.strings(joinPath(op, "uncertaintyRefs"), o.UncertaintyRefs, 1, -1, 0, -1)
		}
		c.strings(joinPath(rp, "evidenceRefs"), r.EvidenceRefs, 1, -1, 0, -1)
		c.str(joinPath(rp, "rationale"), r.Rationale, 1, 600)
		c.str(joinPath(rp, "policyRef"), r.PolicyRef, 1, 120)
	}
	s := p.SupportSummary
	for name, value := range map[string]int{"supported": s.Supported, "conflicting": s.Conflicting, "gap": s.Gap, "undetermined": s.Undetermined, "total": s.Total} {
		c.intRange(joinPath(path, "supportSummary", name), value, 0, 0, false)
	}
	c.strings(joinPath(path, "entryRoutes"), p.EntryRoutes, 1, 300, 0, 6)
	c.arr(joinPath(path, "tradeoffs"), len(p.Tradeoffs), 0, 8)
	for i, t := range p.Tradeoffs {
		tp := idx(joinPath(path, "tradeoffs"), i)
		c.str(joinPath(tp, "statement"), t.Statement, 1, 400)
		c.str(joinPath(tp, "costsWhat"), t.CostsWhat, 1, 300)
		c.str(joinPath(tp, "sourceRef"), t.SourceRef, 1, 200)
	}
	c.strings(joinPath(path, "openQuestions"), p.OpenQuestions, 1, 400, 0, 8)
	c.arr(joinPath(path, "uncertainty"), len(p.Uncertainty), 0, 12)
	for i, note := range p.Uncertainty {
		checkUncertaintyNote(c, idx(joinPath(path, "uncertainty"), i), note)
	}
	c.strings(joinPath(path, "supportingRepresentationRefs"), p.SupportingRepresentationRefs, 1, -1, 1, -1)
	c.strings(joinPath(path, "supportingTransformationRefs"), p.SupportingTransformationRefs, 1, -1, 1, -1)
	c.strings(joinPath(path, "evidenceRefs"), p.EvidenceRefs, 1, -1, 1, -1)
	c.str(joinPath(path, "sourceRef"), p.SourceRef, 1, 200)
	c.enum(joinPath(path, "review", "state"), p.Review.State, PathwayReviewStates)
	checkActor(c, joinPath(path, "review", "decidedBy"), p.Review.DecidedBy)
	checkOptIso(c, joinPath(path, "review", "decidedAt"), p.Review.DecidedAt)
	c.optStr(joinPath(path, "review", "rationale"), p.Review.Rationale, 0, 600)
}

func ValidatePathwayCandidate(p PathwayCandidate) []Issue {
	c := &checker{}
	checkPathwayCandidate(c, "", p)
	return c.result()
}

type PathwayCoverage struct {
	AssessedConstructs   []string `json:"assessedConstructs"`
	UnassessedConstructs []string `json:"unassessedConstructs"`
	ObservationCount     int      `json:"observationCount"`
	RepresentationCount  int      `json:"representationCount"`
}

type PathwaySet struct {
	ID            string   `json:"id"`
	SchemaVersion string   `json:"schemaVersion"`
	SubjectID     string   `json:"subjectId"`
	CreatedAt     string   `json:"createdAt"`
	CreatedBy     ActorRef `json:"createdBy"`
	// Deliberately unordered: the array carries no ranking semantics.
	Pathways []PathwayCandidate `json:"pathways"`
	// What the mapping did not consider, stated explicitly.
	Coverage PathwayCoverage `json:"coverage"`
	// How the mapping was performed and with which policy version.
	MappingRef       string `json:"mappingRef"`
	MappingPolicyRef string `json:"mappingPolicyRef"`
	// The user boundary: this set is information for 0, not a decision by the system.
	UserAuthorityStatement string   `json:"userAuthorityStatement"`
	RestrictedUse          []string `json:"restrictedUse"`
}

func ValidatePathwaySet(set PathwaySet) []Issue {
	c := &checker{}
	PathwaySetIDKind.check(c, "id", set.ID)
	checkSemver(c, "schemaVersion", set.SchemaVersion)
	SubjectIDKind.check(c, "subjectId", set.SubjectID)
	checkIso(c, "createdAt", set.CreatedAt)
	checkActor(c, "createdBy", set.CreatedBy)
	c.arr("pathways", len(set.Pathways), 1, 24)
	for i, p := range set.Pathways {
		checkPathwayCandidate(c, idx("pathways", i), p)
	}
	c.strings("coverage.assessedConstructs", set.Coverage.AssessedConstructs, 1, -1, 0, -1)
	c.strings("coverage.unassessedConstructs", set.Coverage.UnassessedConstructs, 1, -1, 0, -1)
	c.intRange("coverage.observationCount", set.Coverage.ObservationCount, 0, 0, false)
	c.intRange("coverage.representationCount", set.Coverage.RepresentationCount, 0, 0, false)
	c.str("mappingRef", set.MappingRef, 1, 160)
	c.str("mappingPolicyRef", set.MappingPolicyRef, 1, 160)
	c.str("userAuthorityStatement", set.UserAuthorityStatement, 1, 400)
	c.strings("restrictedUse", set.RestrictedUse, 1, 300, 0, 8)
	return c.result()
}

const UserAuthorityStatement = "This is pathway information assembled from your own observations. It is not a recommendation and it does not make your decision: you decide which direction, if any, to pursue."

func SupportSummaryOf(requirements []PathwayRequirement) SupportSummary {
	summary := SupportSummary{Total: len(requirements)}
	for _, r := range requirements {
		switch r.Status {
		case "supported":
			summary.Supported++
		case "conflicting":
			summary.Conflicting++
		case "gap":
			summary.Gap++
		case "undetermined":
			summary.Undetermined++
		}
	}
	return summary
}

// ----------------------------------------------------------------- mapping
//
// M(A, E, C, U) → { P_1 … P_n }. The mapping is criterion-referenced and
// declarative: each requirement is checked against the observations that bear
// on it using bands declared by a versioned policy, and every status carries
// the observations and evidence that produced it. The output is never an
// ordering and never a single answer.
//
// What the mapping explicitly does NOT do:
//   - it does not aggregate observations into a career score (§4);
//   - it does not rank pathways (§7);
//   - it does not decide for the user (§2).

const (
	MapPathwaysTransformationID      = "transformation.map-pathways"
	MapPathwaysTransformationVersion = "1.0.0"
)

type MappingPolicy struct {
	ID       string  `json:"id"`
	Version  string  `json:"version"`
	HighBand float64 `json:"highBand"`
	LowBand  float64 `json:"lowBand"`
	// Uncertainty codes that make a single observation unreadable for mapping.
	BlockingCodes []string `json:"blockingCodes"`
	// Context keys an observation must declare before it may be mapped. Career
	// mapping is explicitly context-dependent (§7): without the setting and the
	// career stage, a reading cannot legitimately be placed in a career
	// direction, so the requirement is reported as undetermined rather than
	// guessed.
	RequiredContextKeys []string `json:"requiredContextKeys"`
	// Uncertainty level at which a blocking code actually blocks.
	BlockingLevel string `json:"blockingLevel"`
	Statement     string `json:"statement"`
}

var DefaultMappingPolicy = MappingPolicy{
	ID:                  "policy.pathway-mapping.criterion-referenced.v1",
	Version:             "1.0.0",
	HighBand:            0.66,
	LowBand:             0.34,
	BlockingCodes:       []string{"U-RESPONSE-MISSING", "U-RESPONSE-AMBIGUOUS"},
	RequiredContextKeys: []string{"setting", "careerStage"},
	BlockingLevel:       LevelModerate,
	Statement:           "Each requirement is read against the user’s own observations using fixed criterion bands. A band reading is a criterion-referenced statement about one observation, not a measurement of fit, not a probability, and not comparable across pathways as a score.",
}

type observation struct {
	representationRef string
	signalRef         string
	evidenceRefs      []string
	uncertaintyRefs   []string
	kind              string
	position          *float64
	selectedOptionIDs []string
	selectedLabels    []string
	displayValue      string
	blocked           bool
	blockingReason    string
}

type MapPathwaysParams struct {
	SubjectID       string
	Representations []Representation1to3
	CreatedAt       string
	CreatedBy       ActorRef
	Catalogue       []PathwayTemplate
	Policy          *MappingPolicy
	// Existing review states, preserved across re-mapping (the user's
	// decisions are not lost).
	ExistingReviews   map[string]PathwayReview
	TransformationRef string
}

func collectObservations(representations []Representation1to3, policy MappingPolicy) map[string][]observation {
	index := map[string][]observation{}
	for _, r := range representations {
		evidenceRefs := make([]string, len(r.E.Bundle))
		for i, record := range r.E.Bundle {
			evidenceRefs[i] = record.ID
		}
		uncertaintyRefs := make([]string, len(r.U.Notes))
		for i, note := range r.U.Notes {
			uncertaintyRefs[i] = note.ID
		}
		var blockingNote *UncertaintyNote
		for i := range r.U.Notes {
			note := r.U.Notes[i]
			if Contains(policy.BlockingCodes, note.Code) && LevelRank(note.Level) >= LevelRank(policy.BlockingLevel) {
				blockingNote = &r.U.Notes[i]
				break
			}
		}
		missing := []string{}
		for _, key := range policy.RequiredContextKeys {
			if _, ok := FindContextField(r.C, key); !ok {
				missing = append(missing, key)
			}
		}
		blocked := blockingNote != nil || r.U.Coverage == "unassessed" || len(missing) > 0
		blockingReason := ""
		switch {
		case blockingNote != nil:
			blockingReason = blockingNote.Detail
		case len(missing) > 0:
			blockingReason = fmt.Sprintf("context does not declare %s, which this mapping policy requires before a reading can be placed in a career direction", strings.Join(missing, ", "))
		case r.U.Coverage == "unassessed":
			blockingReason = "uncertainty was not assessed for this observation"
		}

		for _, construct := range r.A.Constructs {
			o := observation{
				representationRef: r.ID,
				signalRef:         r.SignalID,
				evidenceRefs:      evidenceRefs,
				uncertaintyRefs:   uncertaintyRefs,
				blocked:           blocked,
				blockingReason:    blockingReason,
			}
			switch r.A.Response.Kind {
			case "ordinal":
				o.kind = "ordinal"
				o.position = floatPtr(r.A.Response.PositionValue())
				o.displayValue = fmt.Sprintf("%s (position %s)", r.A.Response.PresentedLabel(), JSFixed(r.A.Response.PositionValue(), 2))
			case "categorical":
				o.kind = "categorical"
				o.selectedOptionIDs = r.A.Response.SelectedOptionIDs
				o.selectedLabels = r.A.Response.SelectedLabels
				o.displayValue = strings.Join(r.A.Response.SelectedLabels, ", ")
			default:
				o.kind = "text"
				o.displayValue = "free-text observation (interpretation proposed separately)"
			}
			index[construct.ConstructID] = append(index[construct.ConstructID], o)
		}
	}
	return index
}

type requirementOutcome struct {
	status       string
	rationale    string
	observations []observation
	evidenceRefs []string
}

func evidenceOf(observations []observation) []string {
	refs := []string{}
	for _, o := range observations {
		refs = append(refs, o.evidenceRefs...)
	}
	return refs
}

func evaluateAlignment(template PathwayRequirementTemplate, observations []observation, policy MappingPolicy) requirementOutcome {
	relevant := []observation{}
	for _, o := range observations {
		if o.kind == "ordinal" && o.position != nil {
			relevant = append(relevant, o)
		}
	}
	if len(relevant) == 0 {
		return requirementOutcome{status: "undetermined", rationale: "No observation in your record bears on this requirement yet.", observations: []observation{}, evidenceRefs: []string{}}
	}
	blocked := []observation{}
	readable := []observation{}
	for _, o := range relevant {
		if o.blocked {
			blocked = append(blocked, o)
		} else {
			readable = append(readable, o)
		}
	}
	if len(blocked) == len(relevant) {
		reasons := make([]string, len(blocked))
		for i, o := range blocked {
			reasons[i] = o.blockingReason
			if reasons[i] == "" {
				reasons[i] = "unreadable"
			}
		}
		return requirementOutcome{
			status:       "undetermined",
			rationale:    "Every observation for this requirement is marked unreadable: " + strings.Join(reasons, "; "),
			observations: relevant,
			evidenceRefs: evidenceOf(relevant),
		}
	}
	direction := template.Direction
	if direction == "" {
		direction = "high"
	}
	high, low, middle := 0, 0, 0
	for _, o := range readable {
		position := 0.0
		if o.position != nil {
			position = *o.position
		}
		switch {
		case position >= policy.HighBand:
			high++
		case position <= policy.LowBand:
			low++
		}
		if position > policy.LowBand && position < policy.HighBand {
			middle++
		}
	}
	outcome := func(status, rationale string) requirementOutcome {
		return requirementOutcome{status: status, rationale: rationale, observations: readable, evidenceRefs: evidenceOf(readable)}
	}
	policyRef := policy.ID + "@" + policy.Version

	if high > 0 && low > 0 {
		return outcome("conflicting", fmt.Sprintf("Your observations for this requirement disagree: %d reading(s) in the high band and %d in the low band. This can indicate a changed situation or a changed reading; the system does not choose between them.", high, low))
	}
	if direction == "any" {
		return outcome("supported", "An observation exists for this requirement and the template only asks whether it is present.")
	}
	towardHigh := direction == "high"
	if high > 0 && towardHigh {
		return outcome("supported", fmt.Sprintf("Reading(s) at or above the high band (%s) of the declared policy %s.", JSFixed(policy.HighBand, 2), policyRef))
	}
	if low > 0 && !towardHigh {
		return outcome("supported", fmt.Sprintf("The template expects a low reading here, and reading(s) are at or below the low band (%s) under policy %s.", JSFixed(policy.LowBand, 2), policyRef))
	}
	if high > 0 && !towardHigh {
		return outcome("gap", fmt.Sprintf("The template expects a low reading here, but reading(s) are in the high band (≥%s) under policy %s.", JSFixed(policy.HighBand, 2), policyRef))
	}
	if low > 0 && towardHigh {
		return outcome("gap", fmt.Sprintf("Reading(s) at or below the low band (%s) under policy %s.", JSFixed(policy.LowBand, 2), policyRef))
	}
	return outcome("undetermined", fmt.Sprintf("Reading(s) fall between the bands (%s–%s), so this observation does not distinguish the requirement either way. %d reading(s) in the middle band.", JSFixed(policy.LowBand, 2), JSFixed(policy.HighBand, 2), middle))
}

func evaluateConstraint(template PathwayRequirementTemplate, observations []observation) requirementOutcome {
	relevant := []observation{}
	for _, o := range observations {
		if o.kind == "categorical" {
			relevant = append(relevant, o)
		}
	}
	if len(relevant) == 0 {
		return requirementOutcome{status: "undetermined", rationale: "No declared constraint observation bears on this requirement yet.", observations: []observation{}, evidenceRefs: []string{}}
	}
	selected := []string{}
	for _, o := range relevant {
		selected = append(selected, o.selectedOptionIDs...)
	}
	outcome := func(status, rationale string) requirementOutcome {
		return requirementOutcome{status: status, rationale: rationale, observations: relevant, evidenceRefs: evidenceOf(relevant)}
	}
	if len(selected) == 0 {
		return outcome("undetermined", "Constraint observation carries no selection.")
	}
	if template.ConstraintAllowedOptions != nil {
		satisfied := false
		for _, optionID := range selected {
			if Contains(template.ConstraintAllowedOptions, optionID) {
				satisfied = true
			}
		}
		if satisfied {
			return outcome("supported", "Your declared availability is compatible with the working pattern this direction normally uses.")
		}
		return outcome("gap", "Your declared availability is not compatible with the working pattern this direction normally uses.")
	}
	if template.ConstraintOptionOrder != nil && template.ConstraintMinRank != nil {
		minRank := *template.ConstraintMinRank
		satisfied := false
		for _, optionID := range selected {
			rank := -1
			for i, candidate := range template.ConstraintOptionOrder {
				if candidate == optionID {
					rank = i
					break
				}
			}
			if rank >= minRank {
				satisfied = true
			}
		}
		if satisfied {
			return outcome("supported", fmt.Sprintf("Your declared mobility covers rank %d or above in the template’s declared ordering.", minRank))
		}
		return outcome("gap", fmt.Sprintf("Your declared mobility does not reach rank %d in the template’s declared ordering.", minRank))
	}
	return outcome("undetermined", "Constraint requirement is not expressible against the recorded observations.")
}

var carriedPathwayCodes = []string{
	"U-INSTRUMENT-UNVALIDATED",
	"U-CONTEXT-UNDECLARED",
	"U-STALE-OBSERVATION",
	"U-RESPONSE-AMBIGUOUS",
	"U-RESPONSE-MISSING",
	"U-SELF-REPORT-BIAS",
	"U-CONTEXT-CONFLICT",
}

func pathwayUncertainty(representations []Representation1to3, declaredLosses []string, transformationRef string, notedBy ActorRef) []UncertaintyNote {
	notes := []UncertaintyNote{}
	seen := map[string]bool{}
	push := func(note UncertaintyNote) {
		key := note.Code + "|" + note.Detail
		if seen[key] {
			return
		}
		seen[key] = true
		notes = append(notes, note)
	}
	for _, r := range representations {
		for _, note := range r.U.Notes {
			// Carry forward only the blocking codes and the quality/validity codes
			// that change how a pathway may be read.
			if Contains(carriedPathwayCodes, note.Code) {
				push(note)
			}
		}
	}
	d := mustDescriptor("U-TRANSFORMATION-LOSS")
	push(UncertaintyNote{
		ID:         MintID("unc", map[string]any{"transformationRef": transformationRef, "losses": declaredLosses}),
		Code:       "U-TRANSFORMATION-LOSS",
		Source:     d.Source,
		Kind:       d.Kind,
		Level:      d.DefaultLevel,
		Detail:     "Mapping to pathway information applies declared losses: " + strings.Join(declaredLosses, "; "),
		Affects:    CopyStrings(d.Affects),
		Mitigation: d.Mitigation,
		NotedBy:    notedBy,
	})
	if len(notes) > 12 {
		notes = notes[:12]
	}
	return notes
}

func restrictedUseFrom(representations []Representation1to3) []string {
	restrictions := []string{}
	add := func(text string) {
		if !Contains(restrictions, text) {
			restrictions = append(restrictions, text)
		}
	}
	for _, r := range representations {
		for _, note := range r.U.Notes {
			switch note.Code {
			case "U-INSTRUMENT-UNVALIDATED":
				add("Readings come from an instrument with no recorded validation evidence: treat requirement statuses as hypotheses to test, not as measurements of ability or suitability.")
			case "U-STALE-OBSERVATION":
				add("Some observations are old relative to this mapping; conditions may have changed since they were recorded.")
			case "U-CONTEXT-UNDECLARED":
				add("Some observations carry thin context, so their relevance to this direction is correspondingly uncertain.")
			}
		}
	}
	add("Templates carry no labour-market data: check vacancies, pay, and local demand independently.")
	if len(restrictions) > 8 {
		restrictions = restrictions[:8]
	}
	return restrictions
}

// MapToPathways maps validated representations onto criterion-referenced
// pathway information.
func MapToPathways(params MapPathwaysParams) (PathwaySet, []Rejection) {
	policy := DefaultMappingPolicy
	if params.Policy != nil {
		policy = *params.Policy
	}
	catalogue := params.Catalogue
	if catalogue == nil {
		catalogue = PathwayCatalogue
	}
	if len(params.Representations) == 0 {
		return PathwaySet{}, []Rejection{NewRejection("S2.PATHWAY.NO-REPRESENTATIONS", "pathway mapping requires at least one validated representation (§S1 → S2)", "representations")}
	}

	observations := collectObservations(params.Representations, policy)
	transformationID := params.TransformationRef
	if transformationID == "" {
		transformationID = MapPathwaysTransformationID
	}
	transformationRef := transformationID + "@" + MapPathwaysTransformationVersion
	declaredLosses := []string{
		"requirement bands replace the continuous reading recorded on the observation",
		"observations are grouped by construct, so within-construct variation is only visible in the per-requirement list",
		"pathway templates state generic directions, not specific employers or vacancies",
	}
	policyRef := policy.ID + "@" + policy.Version

	var rejections []Rejection
	candidates := []PathwayCandidate{}
	for _, template := range catalogue {
		requirements := []PathwayRequirement{}
		for _, rt := range template.Requirements {
			relevant := observations[rt.ConstructID]
			var out requirementOutcome
			if rt.Kind == "constraint" {
				out = evaluateConstraint(rt, relevant)
			} else {
				out = evaluateAlignment(rt, relevant, policy)
			}
			if out.status != "undetermined" && len(out.evidenceRefs) == 0 {
				rejections = append(rejections, NewRejection("INV.PATHWAY.SUPPORT",
					fmt.Sprintf("requirement %s of %s claims status %q with no evidence reference", rt.RequirementID, template.PathwayID, out.status), "requirements"))
			}
			observed := make([]RequirementObservation, len(out.observations))
			for i, o := range out.observations {
				observed[i] = RequirementObservation{
					RepresentationRef: o.representationRef,
					SignalRef:         o.signalRef,
					Value:             o.displayValue,
					EvidenceRefs:      CopyStrings(o.evidenceRefs),
					UncertaintyRefs:   CopyStrings(o.uncertaintyRefs),
				}
			}
			requirements = append(requirements, PathwayRequirement{
				RequirementID: rt.RequirementID,
				ConstructID:   rt.ConstructID,
				Statement:     rt.Statement,
				Kind:          rt.Kind,
				Status:        out.status,
				Observations:  observed,
				EvidenceRefs:  CopyStrings(out.evidenceRefs),
				Rationale:     out.rationale,
				PolicyRef:     policyRef,
			})
		}

		supportingRepresentationRefs := make([]string, len(params.Representations))
		evidenceRefs := []string{}
		for i, r := range params.Representations {
			supportingRepresentationRefs[i] = r.ID
			for _, record := range r.E.Bundle {
				evidenceRefs = append(evidenceRefs, record.ID)
			}
		}
		uncertainty := pathwayUncertainty(params.Representations, declaredLosses, transformationRef, params.CreatedBy)

		pathwayID := MintID("pth", map[string]any{
			"pathwayId":       template.PathwayID,
			"subjectId":       params.SubjectID,
			"representations": supportingRepresentationRefs,
			"policy":          policyRef,
		})

		review, ok := params.ExistingReviews[template.PathwayID]
		if !ok {
			review = PathwayReview{State: "unreviewed", DecidedBy: UserZeroActor}
		}

		tradeoffs := make([]Tradeoff, len(template.Tradeoffs))
		for i, t := range template.Tradeoffs {
			tradeoffs[i] = Tradeoff{Statement: t.Statement, CostsWhat: t.CostsWhat, SourceRef: template.SourceRef}
		}

		candidate := PathwayCandidate{
			ID:                           pathwayID,
			PathwayID:                    template.PathwayID,
			Name:                         template.Name,
			Family:                       template.Family,
			Description:                  template.Description,
			Requirements:                 requirements,
			SupportSummary:               SupportSummaryOf(requirements),
			EntryRoutes:                  CopyStrings(template.EntryRoutes),
			Tradeoffs:                    tradeoffs,
			OpenQuestions:                CopyStrings(template.OpenQuestions),
			Uncertainty:                  uncertainty,
			SupportingRepresentationRefs: supportingRepresentationRefs,
			SupportingTransformationRefs: []string{DecomposeTransformationID + "@" + DecomposeTransformationVersion, transformationRef},
			EvidenceRefs:                 evidenceRefs,
			SourceRef:                    template.SourceRef,
			Review:                       review,
		}
		if issues := ValidatePathwayCandidate(candidate); len(issues) > 0 {
			for _, issue := range issues {
				path := issue.Path
				if path == "" {
					path = "pathway"
				}
				rejections = append(rejections, NewRejection("S2.PATHWAY.SHAPE", template.PathwayID+": "+issue.Message, path))
			}
			continue
		}
		candidates = append(candidates, candidate)
	}

	if len(rejections) > 0 {
		return PathwaySet{}, rejections
	}
	if len(candidates) == 0 {
		return PathwaySet{}, []Rejection{NewRejection("S2.PATHWAY.EMPTY", "no pathway candidate could be constructed", "pathways")}
	}

	assessedSet := map[string]bool{}
	for _, r := range params.Representations {
		for _, construct := range r.A.Constructs {
			assessedSet[construct.ConstructID] = true
		}
	}
	assessed := make([]string, 0, len(assessedSet))
	for id := range assessedSet {
		assessed = append(assessed, id)
	}
	sort.Strings(assessed)
	unassessed := []string{}
	for _, id := range CatalogueConstructIDs() {
		if !assessedSet[id] {
			unassessed = append(unassessed, id)
		}
	}

	representationIDs := make([]string, len(params.Representations))
	for i, r := range params.Representations {
		representationIDs[i] = r.ID
	}
	sortedIDs := CopyStrings(representationIDs)
	sort.Strings(sortedIDs)

	set := PathwaySet{
		ID: MintID("pths", map[string]any{
			"subjectId":       params.SubjectID,
			"representations": sortedIDs,
			"policy":          policyRef,
			"createdAt":       params.CreatedAt,
		}),
		SchemaVersion: PathwaySetSchemaVersion,
		SubjectID:     params.SubjectID,
		CreatedAt:     params.CreatedAt,
		CreatedBy:     params.CreatedBy,
		Pathways:      candidates,
		Coverage: PathwayCoverage{
			AssessedConstructs:   assessed,
			UnassessedConstructs: unassessed,
			ObservationCount:     len(params.Representations),
			RepresentationCount:  len(params.Representations),
		},
		MappingRef:             transformationRef,
		MappingPolicyRef:       policyRef,
		UserAuthorityStatement: UserAuthorityStatement,
		RestrictedUse:          restrictedUseFrom(params.Representations),
	}
	if issues := ValidatePathwaySet(set); len(issues) > 0 {
		return PathwaySet{}, IssuesToRejections("S2.PATHWAY.SET-SHAPE", issues, "pathwaySet")
	}
	return set, nil
}

// ------------------------------------------------------------------ review
//
// The 0 boundary at the end of the loop. The system may assemble pathway
// information. Only the user (0) may mark a pathway as interesting,
// not-a-fit, pursuing, or deferred. Any attempt by a system, capability, or
// model actor to set that state is refused.

type SetPathwayReviewParams struct {
	Set       PathwaySet
	PathwayID string
	State     string
	DecidedBy ActorRef
	DecidedAt string
	Rationale string
}

func SetPathwayReview(params SetPathwayReviewParams) (PathwaySet, []Rejection) {
	if params.DecidedBy.Kind != ActorUser {
		return PathwaySet{}, []Rejection{NewRejection("INV.PATHWAY.USER-AUTHORITY",
			fmt.Sprintf("pathway review state must be decided by the user (0); received a %q actor. The system never chooses a path for the user.", params.DecidedBy.Kind),
			"review.decidedBy")}
	}
	index := -1
	for i, p := range params.Set.Pathways {
		if p.PathwayID == params.PathwayID {
			index = i
			break
		}
	}
	if index < 0 {
		return PathwaySet{}, []Rejection{NewRejection("S4.PATHWAY.UNKNOWN", fmt.Sprintf("no pathway %q in this set", params.PathwayID), "pathways")}
	}
	updated := params.Set
	updated.Pathways = append([]PathwayCandidate{}, params.Set.Pathways...)
	updated.Pathways[index].Review = PathwayReview{
		State:     params.State,
		DecidedBy: params.DecidedBy,
		DecidedAt: params.DecidedAt,
		Rationale: params.Rationale,
	}
	if issues := ValidatePathwaySet(updated); len(issues) > 0 {
		return PathwaySet{}, IssuesToRejections("S4.PATHWAY.REVIEW-SHAPE", issues, "review")
	}
	return updated, nil
}

type ReviewedPathway struct {
	PathwayID string `json:"pathwayId"`
	State     string `json:"state"`
	DecidedAt string `json:"decidedAt,omitempty"`
}

// ReviewedPathways lists user decisions recorded against a set.
func ReviewedPathways(set PathwaySet) []ReviewedPathway {
	out := []ReviewedPathway{}
	for _, p := range set.Pathways {
		if p.Review.State != "unreviewed" {
			out = append(out, ReviewedPathway{PathwayID: p.PathwayID, State: p.Review.State, DecidedAt: p.Review.DecidedAt})
		}
	}
	return out
}

type DecisionSummary struct {
	Total      int `json:"total"`
	Unreviewed int `json:"unreviewed"`
	Explored   int `json:"explored"`
	Declined   int `json:"declined"`
	Pursuing   int `json:"pursuing"`
}

// UserDecisionSummary never reports a "leading" or "best" pathway. It reports
// counts of user decisions.
func UserDecisionSummary(set PathwaySet) DecisionSummary {
	summary := DecisionSummary{Total: len(set.Pathways)}
	for _, p := range set.Pathways {
		switch p.Review.State {
		case "unreviewed":
			summary.Unreviewed++
		case "interesting", "deferred":
			summary.Explored++
		case "not-a-fit":
			summary.Declined++
		case "pursuing":
			summary.Pursuing++
		}
	}
	return summary
}
