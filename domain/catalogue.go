package domain

import "sort"

// Pathway templates.
//
// Status: PROJECT TEMPLATES, NOT LABOUR-MARKET DATA. Each template describes a
// career direction in terms of requirements that can be checked against the
// user's own observations, plus entry routes, trade-offs, and open questions.
// Templates carry no ranking, no popularity, and no implied fit: they exist so
// that mapping produces explainable information (§7).
//
// SourceRef on every template names where the template came from. In this
// deployment they are project-authored, and say so — they are not presented
// as labour-market statistics.

type PathwayRequirementTemplate struct {
	RequirementID string `json:"requirementId"`
	ConstructID   string `json:"constructId"`
	ConstructKind string `json:"constructKind"`
	Statement     string `json:"statement"`
	Kind          string `json:"kind"`
	// Ordinal criteria are criterion-referenced bands declared by the
	// template, checked against single observations: "high", "low" or "any".
	Direction string `json:"direction,omitempty"`
	// For constraint criteria: ordered option ids and the minimum acceptable rank.
	ConstraintOptionOrder []string `json:"constraintOptionOrder,omitempty"`
	ConstraintMinRank     *int     `json:"constraintMinRank,omitempty"`
	// For constraint criteria: acceptable option ids (no ordering implied).
	ConstraintAllowedOptions []string `json:"constraintAllowedOptions,omitempty"`
}

type TemplateTradeoff struct {
	Statement string `json:"statement"`
	CostsWhat string `json:"costsWhat"`
}

type PathwayTemplate struct {
	PathwayID     string                       `json:"pathwayId"`
	Name          string                       `json:"name"`
	Family        string                       `json:"family"`
	Description   string                       `json:"description"`
	Requirements  []PathwayRequirementTemplate `json:"requirements"`
	EntryRoutes   []string                     `json:"entryRoutes"`
	Tradeoffs     []TemplateTradeoff           `json:"tradeoffs"`
	OpenQuestions []string                     `json:"openQuestions"`
	SourceRef     string                       `json:"sourceRef"`
	// What this template is not able to consider, stated up front.
	RestrictedUse []string `json:"restrictedUse"`
}

const templateSource = "project-template:career107.pathway-catalogue@0.1.0"

// PathwayCatalogueRef names the catalogue the templates come from.
const PathwayCatalogueRef = templateSource

var mobilityOrder = []string{
	"opt.remote-only",
	"opt.local",
	"opt.regional",
	"opt.relocate-national",
	"opt.relocate-international",
}

var allAvailability = []string{
	"opt.standard-weekdays",
	"opt.shift-rotating",
	"opt.evenings-weekends",
	"opt.irregular",
	"opt.fully-flexible",
}

func alignment(id, constructID, constructKind, statement, direction string) PathwayRequirementTemplate {
	return PathwayRequirementTemplate{RequirementID: id, ConstructID: constructID, ConstructKind: constructKind, Statement: statement, Kind: "alignment", Direction: direction}
}

func experience(id, constructID, statement, direction string) PathwayRequirementTemplate {
	return PathwayRequirementTemplate{RequirementID: id, ConstructID: constructID, ConstructKind: "experience", Statement: statement, Kind: "experience", Direction: direction}
}

func scheduleConstraint(id, statement string, allowed []string) PathwayRequirementTemplate {
	return PathwayRequirementTemplate{RequirementID: id, ConstructID: "constraint.schedule", ConstructKind: "constraint", Statement: statement, Kind: "constraint", ConstraintAllowedOptions: allowed}
}

func mobilityConstraint(id, statement string, minRank int) PathwayRequirementTemplate {
	return PathwayRequirementTemplate{RequirementID: id, ConstructID: "constraint.geography", ConstructKind: "constraint", Statement: statement, Kind: "constraint", ConstraintOptionOrder: mobilityOrder, ConstraintMinRank: intPtr(minRank)}
}

var PathwayCatalogue = []PathwayTemplate{
	{
		PathwayID:   "path.data-analysis",
		Name:        "Data analysis and reporting",
		Family:      "analysis",
		Description: "Turning collected information into readings other people can act on: cleaning, examining, and reporting data in an organisational setting.",
		Requirements: []PathwayRequirementTemplate{
			alignment("req.data-analysis.interest", "interest.analysis", "interest", "Reported interest in examining information for patterns.", "high"),
			alignment("req.data-analysis.quantitative", "skill.quantitative", "skill-self-report", "Self-reported comfort drawing conclusions from numbers.", "high"),
			alignment("req.data-analysis.structure", "style.structure-preference", "work-style", "Tolerance for defined, repeatable analytical procedure.", "any"),
			scheduleConstraint("req.data-analysis.availability", "Availability consistent with scheduled reporting cycles.", []string{"opt.standard-weekdays", "opt.fully-flexible", "opt.evenings-weekends"}),
		},
		EntryRoutes: []string{
			"Internal move from a role that already produces data (operations, support, finance administration).",
			"Structured short course in data handling, followed by a portfolio of analyses on your own or public datasets.",
			"Assistant or junior analyst posting that accepts adjacent experience.",
		},
		Tradeoffs: []TemplateTradeoff{
			{Statement: "Analytical work is often adjacent to decisions rather than making them.", CostsWhat: "Visible authority over outcomes."},
			{Statement: "Reporting deadlines recur on a fixed cycle.", CostsWhat: "Spontaneity in how the week is shaped."},
		},
		OpenQuestions: []string{
			"Would you rather define the questions or answer the questions someone else defined?",
			"How do you feel about cleaning data as most of the work?",
		},
		SourceRef: templateSource,
		RestrictedUse: []string{
			"The template describes a direction, not an opening: it says nothing about current vacancies, pay, or regional demand.",
		},
	},
	{
		PathwayID:   "path.software-engineering",
		Name:        "Software engineering",
		Family:      "building",
		Description: "Designing and maintaining software systems, usually in a team, with continuing maintenance responsibilities.",
		Requirements: []PathwayRequirementTemplate{
			alignment("req.software.creation", "interest.creation", "interest", "Reported interest in producing artifacts that did not exist before.", "high"),
			alignment("req.software.depth", "style.depth", "work-style", "Willingness to go deep in one technical area.", "high"),
			alignment("req.software.tooling", "skill.tooling", "skill-self-report", "Self-reported capability with software tools.", "high"),
			alignment("req.software.collaboration", "style.collaboration", "work-style", "Work is normally reviewed and negotiated with other people.", "any"),
		},
		EntryRoutes: []string{
			"Self-directed build record plus a structured curriculum in fundamentals.",
			"Apprenticeship, conversion programme, or entry-level engineering role open to adjacent experience.",
			"Adjacent move from support, testing, or operations work into a team that already knows you.",
		},
		Tradeoffs: []TemplateTradeoff{
			{Statement: "Maintenance of existing systems occupies much of the work.", CostsWhat: "Time spent on new construction."},
			{Statement: "Technical depth is rewarded slowly and unevenly early on.", CostsWhat: "Immediate recognition."},
		},
		OpenQuestions: []string{
			"Do you want to own systems over years, or build and hand over?",
			"How do you respond to work that is mostly reading existing code?",
		},
		SourceRef: templateSource,
		RestrictedUse: []string{
			"Self-reported tooling capability is unverified: a coding exercise is the only way to establish it, and this system does not administer one.",
		},
	},
	{
		PathwayID:   "path.systems-support",
		Name:        "IT systems and support",
		Family:      "operations",
		Description: "Keeping technical systems working for the people who depend on them, with a strong diagnostic and service component.",
		Requirements: []PathwayRequirementTemplate{
			alignment("req.support.social", "interest.social", "interest", "Reported interest in work centred on other people’s immediate needs.", "high"),
			alignment("req.support.analysis", "interest.analysis", "interest", "Reported interest in diagnosing causes rather than symptoms.", "high"),
			alignment("req.support.explanation", "interest.explanation", "interest", "Reported interest in explaining technical matters to non-specialists.", "high"),
			scheduleConstraint("req.support.availability", "Availability compatible with service coverage patterns.", allAvailability),
		},
		EntryRoutes: []string{
			"Service desk or first-line support role with structured escalation paths.",
			"Certification route in a specific platform you already use.",
			"Internal transfer from a non-technical role in an organisation with a support function.",
		},
		Tradeoffs: []TemplateTradeoff{
			{Statement: "Work is interrupt-driven and prioritised by other people.", CostsWhat: "Control over the shape of the day."},
			{Statement: "Service targets are explicit and measured.", CostsWhat: "Freedom from external metrics."},
		},
		OpenQuestions: []string{
			"How do you feel about being interrupted mid-task several times an hour?",
			"Do you want to fix the symptom quickly or find the cause slowly?",
		},
		SourceRef:     templateSource,
		RestrictedUse: []string{"Template covers service-oriented support; it does not cover engineering-grade infrastructure work."},
	},
	{
		PathwayID:   "path.product-coordination",
		Name:        "Product and project coordination",
		Family:      "coordination",
		Description: "Coordinating what gets built and in which order, typically between users, technical teams, and commercial constraints.",
		Requirements: []PathwayRequirementTemplate{
			alignment("req.product.organisation", "interest.organisation", "interest", "Reported interest in sequencing and coordinating work.", "high"),
			alignment("req.product.collaboration", "style.collaboration", "work-style", "Work happens through conversation and negotiation.", "high"),
			alignment("req.product.facilitation", "skill.facilitation", "skill-self-report", "Self-reported capability guiding groups toward a decision.", "high"),
			alignment("req.product.impact", "value.impact", "value", "Importance placed on visible consequence for users.", "high"),
		},
		EntryRoutes: []string{
			"Associate product or project coordination role inside an existing delivery team.",
			"Move from delivery, support, or analysis work where you already coordinate informally.",
			"Structured programme with a placement in a coordinating role.",
		},
		Tradeoffs: []TemplateTradeoff{
			{Statement: "Accountability is often broader than direct authority.", CostsWhat: "The ability to decide alone."},
			{Statement: "Attention is divided across several workstreams at once.", CostsWhat: "Long uninterrupted depth."},
		},
		OpenQuestions: []string{
			"How do you respond when you are accountable for work you cannot direct?",
			"Would you rather hold the plan or hold the craft?",
		},
		SourceRef:     templateSource,
		RestrictedUse: []string{"Coordination requirements vary widely by organisation; the template states a common shape, not a standard."},
	},
	{
		PathwayID:   "path.research-evaluation",
		Name:        "Research and evaluation",
		Family:      "analysis",
		Description: "Designing studies or evaluations, collecting evidence, and reporting what the evidence can and cannot support.",
		Requirements: []PathwayRequirementTemplate{
			alignment("req.research.analysis", "interest.analysis", "interest", "Reported interest in examining evidence for patterns.", "high"),
			alignment("req.research.writing", "skill.writing", "skill-self-report", "Self-reported capability conveying a complicated point in writing.", "high"),
			alignment("req.research.depth", "style.depth", "work-style", "Sustained engagement with one question over months.", "high"),
			experience("req.research.applied", "experience.applied-projects", "Track record of carrying an enquiry through to a written outcome.", "high"),
		},
		EntryRoutes: []string{
			"Research assistant or evaluation officer role within an organisation or funded project.",
			"Postgraduate route in a relevant method discipline.",
			"Move from data analysis into a team that designs its own studies.",
		},
		Tradeoffs: []TemplateTradeoff{
			{Statement: "Findings frequently cannot support the conclusion the commissioner wants.", CostsWhat: "The comfort of a clean answer."},
			{Statement: "Work is often fixed-term and funding-dependent.", CostsWhat: "Continuity."},
		},
		OpenQuestions: []string{
			`How do you feel about publishing a result that says "we cannot tell"?`,
			"Do you want to design the study or execute a design someone else made?",
		},
		SourceRef:     templateSource,
		RestrictedUse: []string{"Formal research routes commonly require credentials this system does not assess or verify."},
	},
	{
		PathwayID:   "path.teaching-training",
		Name:        "Teaching and training",
		Family:      "people",
		Description: "Designing and delivering instruction, and assessing whether understanding was actually transferred.",
		Requirements: []PathwayRequirementTemplate{
			alignment("req.teaching.explanation", "interest.explanation", "interest", "Reported interest in explaining until someone understands.", "high"),
			alignment("req.teaching.social", "interest.social", "interest", "Reported interest in work centred on other people.", "high"),
			alignment("req.teaching.facilitation", "skill.facilitation", "skill-self-report", "Self-reported capability holding a group’s attention and participation.", "high"),
			scheduleConstraint("req.teaching.availability", "Availability compatible with fixed session timetables.", []string{"opt.standard-weekdays", "opt.shift-rotating", "opt.evenings-weekends", "opt.fully-flexible"}),
		},
		EntryRoutes: []string{
			"Instructional or training role in an organisation that already delivers training internally.",
			"Qualified teaching route with a placement component.",
			"Freelance or platform-based instruction built from existing subject expertise.",
		},
		Tradeoffs: []TemplateTradeoff{
			{Statement: "Delivery is scheduled and repeated.", CostsWhat: "Variety in the working week."},
			{Statement: "Outcomes depend on other people’s effort.", CostsWhat: "A direct link between your work and the result."},
		},
		OpenQuestions: []string{
			"Do you want to work with children, adults, or professionals?",
			"How do you feel about repeating the same material many times per year?",
		},
		SourceRef:     templateSource,
		RestrictedUse: []string{"Regulated teaching contexts impose credential requirements that this system neither checks nor represents."},
	},
	{
		PathwayID:   "path.operations-coordination",
		Name:        "Operations and logistics coordination",
		Family:      "operations",
		Description: "Keeping a physical or service operation running: scheduling, sequencing, and resolving constraints as they appear.",
		Requirements: []PathwayRequirementTemplate{
			alignment("req.ops.organisation", "interest.organisation", "interest", "Reported interest in planning and sequencing.", "high"),
			alignment("req.ops.structure", "style.structure-preference", "work-style", "Comfort with defined procedures and measurable service levels.", "high"),
			scheduleConstraint("req.ops.availability", "Availability compatible with shift or rotating coverage.", allAvailability),
			mobilityConstraint("req.ops.mobility", "Able to attend a physical site.", 1),
		},
		EntryRoutes: []string{
			"Coordinator or scheduler role in a depot, clinic, warehouse, or service centre.",
			"Move from front-line operational work into planning.",
			"Team-leader route inside an operation you already know.",
		},
		Tradeoffs: []TemplateTradeoff{
			{Statement: "Escalations arrive at the least convenient moment.", CostsWhat: "Predictability of the working day."},
			{Statement: "Success is often invisible when nothing goes wrong.", CostsWhat: "Recognition."},
		},
		OpenQuestions: []string{
			"Do you want to optimise a running operation or design a new one?",
			"How do you handle a constraint that cannot be met?",
		},
		SourceRef:     templateSource,
		RestrictedUse: []string{"Physical-site requirements vary by employer; the template states a common pattern only."},
	},
	{
		PathwayID:   "path.creative-production",
		Name:        "Creative production",
		Family:      "building",
		Description: "Producing designed or media artifacts to a brief, usually against deadlines and with review cycles.",
		Requirements: []PathwayRequirementTemplate{
			alignment("req.creative.creation", "interest.creation", "interest", "Reported interest in producing artifacts.", "high"),
			alignment("req.creative.autonomy", "value.autonomy", "value", "Importance of directing how the work is done.", "high"),
			alignment("req.creative.resilience", "style.structure-preference", "work-style", "Capacity to work against a brief that is not yours.", "low"),
			alignment("req.creative.depth", "style.depth", "work-style", "Specialisation in one medium or discipline.", "any"),
		},
		EntryRoutes: []string{
			"Portfolio-led entry: a body of work that demonstrates range and reliability.",
			"Assistant or junior role inside a studio, agency, or production team.",
			"Commission-by-commission freelance route alongside other income.",
		},
		Tradeoffs: []TemplateTradeoff{
			{Statement: "Income is often irregular, particularly early.", CostsWhat: "Financial predictability."},
			{Statement: "Client feedback can override your judgment on the work.", CostsWhat: "Authorial control."},
		},
		OpenQuestions: []string{
			"Can you separate your judgment of the work from your judgment of yourself?",
			"How much of your output are you willing to produce to someone else’s brief?",
		},
		SourceRef:     templateSource,
		RestrictedUse: []string{"Portfolio quality is the decisive factor in this direction and this system does not evaluate portfolios."},
	},
	{
		PathwayID:   "path.trades-maintenance",
		Name:        "Trades and maintenance",
		Family:      "physical",
		Description: "Inspection, repair, and installation work on physical systems, frequently at customer sites.",
		Requirements: []PathwayRequirementTemplate{
			alignment("req.trades.building", "interest.building", "interest", "Reported interest in working with tools, equipment, or material.", "high"),
			experience("req.trades.experience", "experience.applied-projects", "Track record of finishing practical work.", "high"),
			mobilityConstraint("req.trades.mobility", "Able to travel to sites.", 1),
			scheduleConstraint("req.trades.availability", "Availability compatible with early starts and call-out work.", allAvailability),
		},
		EntryRoutes: []string{
			"Apprenticeship with a licensed practitioner.",
			"Technician role with structured on-the-job training.",
			"Government or sector skills programme with a certification endpoint.",
		},
		Tradeoffs: []TemplateTradeoff{
			{Statement: "Physical demands accumulate over a career.", CostsWhat: "Long-term physical capacity."},
			{Statement: "Certification and licensing gates progress.", CostsWhat: "Time before independent practice."},
		},
		OpenQuestions: []string{
			"Do you want to work in customers’ homes and businesses?",
			"How do you feel about certification requirements before you can work independently?",
		},
		SourceRef:     templateSource,
		RestrictedUse: []string{"Licensing requirements are jurisdiction-specific and are not represented here."},
	},
	{
		PathwayID:   "path.customer-facing-services",
		Name:        "Customer-facing services",
		Family:      "people",
		Description: "Front-line work where the primary output is how another person was handled: service, sales support, care-adjacent reception and advice roles.",
		Requirements: []PathwayRequirementTemplate{
			alignment("req.front.social", "interest.social", "interest", "Reported interest in work centred on other people.", "high"),
			alignment("req.front.explanation", "interest.explanation", "interest", "Reported willingness to explain and clarify repeatedly.", "high"),
			alignment("req.front.stability", "value.stability", "value", "Importance of predictable conditions and schedule.", "any"),
			experience("req.front.experience", "experience.applied-projects", "Evidence of finishing work that other people depended on.", "any"),
		},
		EntryRoutes: []string{
			"Direct entry to a front-line role with structured training.",
			"Move from a back-office function into a customer-facing team.",
			"Supervisor route after demonstrated reliability in a front-line role.",
		},
		Tradeoffs: []TemplateTradeoff{
			{Statement: "Emotional labour is a real part of the work.", CostsWhat: "Composure."},
			{Statement: "Performance is visible and measured continuously.", CostsWhat: "Privacy in how you work."},
		},
		OpenQuestions: []string{
			"How do you recover after a difficult interaction?",
			"Do you want the customer relationship or the case behind it?",
		},
		SourceRef:     templateSource,
		RestrictedUse: []string{"Templates do not represent pay, hours, or employment quality; those must be checked with employers."},
	},
}

func GetPathwayTemplate(pathwayID string) (PathwayTemplate, bool) {
	for _, template := range PathwayCatalogue {
		if template.PathwayID == pathwayID {
			return template, true
		}
	}
	return PathwayTemplate{}, false
}

// CatalogueConstructIDs lists the constructs any template can currently
// reference, sorted.
func CatalogueConstructIDs() []string {
	seen := map[string]bool{}
	ids := []string{}
	for _, template := range PathwayCatalogue {
		for _, requirement := range template.Requirements {
			if !seen[requirement.ConstructID] {
				seen[requirement.ConstructID] = true
				ids = append(ids, requirement.ConstructID)
			}
		}
	}
	sort.Strings(ids)
	return ids
}
