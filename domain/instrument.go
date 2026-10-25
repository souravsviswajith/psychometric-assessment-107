package domain

// The project's first instrument: a self-description instrument.
//
// Status: DRAFT INSTRUMENT, NOT A VALIDATED PSYCHOMETRIC MEASURE. Every item is
// marked validated: false unless a validation reference is supplied. The item
// text speaks to the user in ordinary career-assessment language; the
// representation of the answer is technical (§14).
//
// Items are versioned. Changing item wording requires a new item version,
// because the exact wording is part of what was observed.

const SelfDescriptionInstrumentID = "instrument.self-description"

const selfDescriptionVersion = "0.2.0"
const itemVersion = "1.0.0"

func ascending(statement, id string) InterpretationPolicy {
	if id == "" {
		id = "policy.ordinal-ascending.v1"
	}
	return InterpretationPolicy{ID: id, Version: "1.0.0", Direction: "ascending", Statement: statement}
}

var likertAgreement = []InstrumentOption{
	{ID: "opt.not-like-me", Label: "Does not describe me"},
	{ID: "opt.slightly", Label: "Describes me slightly"},
	{ID: "opt.partly", Label: "Describes me partly"},
	{ID: "opt.mostly", Label: "Describes me mostly"},
	{ID: "opt.fully", Label: "Describes me fully"},
}

var frequency5 = []InstrumentOption{
	{ID: "opt.never", Label: "Never"},
	{ID: "opt.rarely", Label: "Rarely"},
	{ID: "opt.sometimes", Label: "Sometimes"},
	{ID: "opt.often", Label: "Often"},
	{ID: "opt.consistently", Label: "Consistently"},
}

var itemLimits = []string{
	"Self-report at a single sitting; reflects the user’s account of themselves at that moment.",
	"No validation evidence is recorded for this item in this deployment.",
}

func likertItem(itemRef, promptText, constructID string) InstrumentItem {
	return InstrumentItem{
		InstrumentID:      SelfDescriptionInstrumentID,
		InstrumentVersion: selfDescriptionVersion,
		ItemRef:           itemRef,
		ItemVersion:       itemVersion,
		PromptText:        promptText,
		ResponseShape:     "likert-5",
		Options:           likertAgreement,
		Policy:            ascending(`The presentation order runs from "does not describe me" to "describes me fully".`, ""),
		Constructs:        []ConstructRef{mustConstructRef(constructID)},
		Validated:         false,
		Limitations:       itemLimits,
	}
}

func frequencyItem(itemRef, promptText, constructID string) InstrumentItem {
	return InstrumentItem{
		InstrumentID:      SelfDescriptionInstrumentID,
		InstrumentVersion: selfDescriptionVersion,
		ItemRef:           itemRef,
		ItemVersion:       itemVersion,
		PromptText:        promptText,
		ResponseShape:     "frequency-5",
		Options:           frequency5,
		Policy:            ascending(`The presentation order runs from "never" to "consistently".`, ""),
		Constructs:        []ConstructRef{mustConstructRef(constructID)},
		Validated:         false,
		Limitations:       itemLimits,
	}
}

func choiceItem(itemRef, promptText string, options []InstrumentOption, constructID string, limitations []string) InstrumentItem {
	return InstrumentItem{
		InstrumentID:      SelfDescriptionInstrumentID,
		InstrumentVersion: selfDescriptionVersion,
		ItemRef:           itemRef,
		ItemVersion:       itemVersion,
		PromptText:        promptText,
		ResponseShape:     "single-select",
		Options:           options,
		Policy:            ascending("Options are unordered; the selection is a categorical constraint, not a magnitude.", "policy.catalogue-selection.v1"),
		Constructs:        []ConstructRef{mustConstructRef(constructID)},
		Validated:         false,
		Limitations:       limitations,
	}
}

var SelfDescriptionV1 = InstrumentDefinition{
	InstrumentID:      SelfDescriptionInstrumentID,
	InstrumentVersion: selfDescriptionVersion,
	Title:             "Self-description instrument (project draft)",
	Status:            "draft-unvalidated",
	Items: []InstrumentItem{
		likertItem("itm-selfdesc-analysis-1", "Working through a problem until the pattern becomes clear is satisfying to me.", "interest.analysis"),
		likertItem("itm-selfdesc-creation-1", "I like making things that did not exist before, whether physical or digital.", "interest.creation"),
		likertItem("itm-selfdesc-social-1", "Work that consists mainly of other people’s questions and needs holds my attention.", "interest.social"),
		likertItem("itm-selfdesc-organisation-1", "I enjoy putting scattered pieces of work into a plan other people can follow.", "interest.organisation"),
		likertItem("itm-selfdesc-building-1", "I prefer work where I can handle tools, equipment, or physical material.", "interest.building"),
		likertItem("itm-selfdesc-explanation-1", "Explaining something until someone else understands it is worth the effort to me.", "interest.explanation"),
		likertItem("itm-selfdesc-autonomy-1", "I need to decide how my own work is done, not only what is delivered.", "value.autonomy"),
		likertItem("itm-selfdesc-impact-1", "I would rather see the consequences of my work on people than on charts.", "value.impact"),
		likertItem("itm-selfdesc-stability-1", "Predictable conditions matter to me more than the size of the opportunity.", "value.stability"),
		likertItem("itm-selfdesc-growth-1", "I want each year of work to leave me more capable than the last.", "value.growth"),
		likertItem("itm-selfdesc-structure-1", "I do better with a defined set of tasks than with an open brief.", "style.structure-preference"),
		likertItem("itm-selfdesc-collaboration-1", "I do my best work in conversation with other people.", "style.collaboration"),
		likertItem("itm-selfdesc-depth-1", "I would rather go deep in one area than stay broadly involved.", "style.depth"),
		likertItem("itm-skill-writing-1", "Writing is a reliable way for me to convey a complicated point.", "skill.writing"),
		likertItem("itm-skill-quantitative-1", "I am comfortable drawing conclusions from numbers.", "skill.quantitative"),
		frequencyItem("itm-experience-projects-1", "In the last two years, how often have you carried a piece of work through to a finished outcome?", "experience.applied-projects"),
		choiceItem(
			"itm-context-availability-1",
			"Which working pattern actually fits your life right now?",
			[]InstrumentOption{
				{ID: "opt.standard-weekdays", Label: "Standard weekdays"},
				{ID: "opt.shift-rotating", Label: "Shift or rotating pattern"},
				{ID: "opt.evenings-weekends", Label: "Evenings and weekends only"},
				{ID: "opt.irregular", Label: "Irregular and unpredictable"},
				{ID: "opt.fully-flexible", Label: "Fully flexible"},
			},
			"constraint.schedule",
			[]string{"Availability is a declared condition of the user’s situation, not a capability or preference assessment."},
		),
		choiceItem(
			"itm-context-mobility-1",
			"How far can you realistically travel for work?",
			[]InstrumentOption{
				{ID: "opt.remote-only", Label: "Remote only"},
				{ID: "opt.local", Label: "Local commute, under 45 minutes"},
				{ID: "opt.regional", Label: "Regional, up to 90 minutes"},
				{ID: "opt.relocate-national", Label: "Willing to relocate within the country"},
				{ID: "opt.relocate-international", Label: "Open to international relocation"},
			},
			"constraint.geography",
			[]string{"Travelled distance is recorded as a mobility band; residence is not recorded or inferred."},
		),
		{
			InstrumentID:      SelfDescriptionInstrumentID,
			InstrumentVersion: selfDescriptionVersion,
			ItemRef:           "itm-narrative-work-1",
			ItemVersion:       itemVersion,
			PromptText:        "Describe one piece of work you would gladly do again, and say what made it worth doing.",
			ResponseShape:     "open-text",
			Options:           []InstrumentOption{},
			Policy: ascending(
				"Free text is kept verbatim as evidence; its interpretation is proposed separately and marked as proposed.",
				"policy.open-text-verbatim.v1",
			),
			Constructs: []ConstructRef{mustConstructRef("experience.applied-projects")},
			Validated:  false,
			Limitations: append([]string{
				"Free-text responses are open to multiple readings; any extraction from them is a proposal, not an observation.",
			}, itemLimits...),
		},
	},
}

// InstrumentRegistry holds every registered instrument, in registration order.
var InstrumentRegistry = []InstrumentDefinition{SelfDescriptionV1}

func GetInstrument(instrumentID string) (InstrumentDefinition, bool) {
	for _, instrument := range InstrumentRegistry {
		if instrument.InstrumentID == instrumentID {
			return instrument, true
		}
	}
	return InstrumentDefinition{}, false
}

func GetItem(instrumentID, itemRef string) (InstrumentItem, bool) {
	instrument, ok := GetInstrument(instrumentID)
	if !ok {
		return InstrumentItem{}, false
	}
	for _, item := range instrument.Items {
		if item.ItemRef == itemRef {
			return item, true
		}
	}
	return InstrumentItem{}, false
}

func FindItem(itemRef string) (InstrumentItem, bool) {
	for _, instrument := range InstrumentRegistry {
		for _, item := range instrument.Items {
			if item.ItemRef == itemRef {
				return item, true
			}
		}
	}
	return InstrumentItem{}, false
}

// AllItems returns every instrument item, flattened — used by the web
// projection and the API.
func AllItems() []InstrumentItem {
	var items []InstrumentItem
	for _, instrument := range InstrumentRegistry {
		items = append(items, instrument.Items...)
	}
	return items
}
