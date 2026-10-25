package domain

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// E of the 1:3 representation.
//
// E answers: why does this observation exist, and what supports its
// provenance? It is structured as E_i = (P_i, M_i, T_i, Q_i) — provenance,
// measurement method, temporal information, evidence quality — so that a
// reviewer can recover source, method, timestamp, version, and quality without
// asking the system that produced the claim (§13, experiment B).

var EvidenceSourceKinds = []string{
	"direct-user-response",
	"user-upload",
	"instrument-administration",
	"third-party-report",
	"system-observation",
	"derived",
}

// Provenance (P): where the observation came from and who is accountable.
type Provenance struct {
	// Stable reference to the originating material (item, upload, protocol step).
	SourceRef  string `json:"sourceRef"`
	SourceKind string `json:"sourceKind"`
	// Who asserted the claim. Users assert; capabilities and models propose.
	AssertedBy ActorRef `json:"assertedBy"`
	// Who recorded it into the system.
	CapturedBy ActorRef `json:"capturedBy"`
	// Previous evidence this record derives from (empty for a primary observation).
	Lineage []string `json:"lineage"`
}

var MeasurementKinds = []string{
	"self-report-item",
	"free-text-response",
	"uploaded-document",
	"interview-protocol",
	"system-log",
	"aggregation",
	"model-extraction",
}

// MeasurementMethod (M): how the observation was collected.
type MeasurementMethod struct {
	// Stable method reference, e.g. "method.self-report-item.v1".
	Ref         string         `json:"ref"`
	Kind        string         `json:"kind"`
	Version     string         `json:"version"`
	Description string         `json:"description"`
	Instrument  *InstrumentRef `json:"instrument,omitempty"`
	// Limits of the method, carried with the evidence rather than discovered later.
	Limitations []string `json:"limitations"`
}

// TemporalRecord (T): temporal information about the observation.
type TemporalRecord struct {
	// When the observation occurred (user time).
	ObservedAt string `json:"observedAt"`
	// When the system recorded it.
	RecordedAt string `json:"recordedAt"`
	// When the observation becomes reviewable again.
	ReviewAt   string `json:"reviewAt,omitempty"`
	SessionRef string `json:"sessionRef,omitempty"`
	// Ordinal position within a session.
	Sequence *int `json:"sequence,omitempty"`
}

var RecencyBands = []string{"current", "recent", "stale", "unknown"}

// EvidenceQuality (Q). Reliability is as claimed by the method, never as
// asserted by the system.
type EvidenceQuality struct {
	Directness   string `json:"directness"`
	Recency      string `json:"recency"`
	Completeness string `json:"completeness"`
	Reliability  string `json:"reliability"`
	Validity     string `json:"validity"`
	// Number of observations the record summarises (1 for a single observation).
	SampleSize int    `json:"sampleSize"`
	Notes      string `json:"notes,omitempty"`
}

type EvidenceRecord struct {
	ID            string            `json:"id"`
	SignalID      string            `json:"signalId"`
	SchemaVersion string            `json:"schemaVersion"`
	Status        string            `json:"status"`
	Provenance    Provenance        `json:"provenance"`
	Method        MeasurementMethod `json:"method"`
	Temporal      TemporalRecord    `json:"temporal"`
	Quality       EvidenceQuality   `json:"quality"`
}

const EvidenceSchemaVersion = "1.0.0"

var observationRefPattern = regexp.MustCompile(`^(sig|evd)_[a-z0-9][a-z0-9-]{1,79}$`)

func checkEvidenceRecord(c *checker, path string, e EvidenceRecord) {
	c.match(joinPath(path, "id"), e.ID, observationRefPattern, "observation references must be sig_/evd_ identifiers")
	SignalIDKind.check(c, joinPath(path, "signalId"), e.SignalID)
	checkSemver(c, joinPath(path, "schemaVersion"), e.SchemaVersion)
	c.enum(joinPath(path, "status"), e.Status, []string{"primary", "derived"})

	p := joinPath(path, "provenance")
	c.str(joinPath(p, "sourceRef"), e.Provenance.SourceRef, 1, 200)
	c.enum(joinPath(p, "sourceKind"), e.Provenance.SourceKind, EvidenceSourceKinds)
	checkActor(c, joinPath(p, "assertedBy"), e.Provenance.AssertedBy)
	checkActor(c, joinPath(p, "capturedBy"), e.Provenance.CapturedBy)
	c.strings(joinPath(p, "lineage"), e.Provenance.Lineage, 1, -1, 0, 8)

	m := joinPath(path, "method")
	c.str(joinPath(m, "ref"), e.Method.Ref, 1, 120)
	c.enum(joinPath(m, "kind"), e.Method.Kind, MeasurementKinds)
	checkSemver(c, joinPath(m, "version"), e.Method.Version)
	c.str(joinPath(m, "description"), e.Method.Description, 1, 400)
	if e.Method.Instrument != nil {
		checkInstrumentRef(c, joinPath(m, "instrument"), *e.Method.Instrument)
	}
	c.strings(joinPath(m, "limitations"), e.Method.Limitations, 1, 300, 0, 6)

	t := joinPath(path, "temporal")
	checkIso(c, joinPath(t, "observedAt"), e.Temporal.ObservedAt)
	checkIso(c, joinPath(t, "recordedAt"), e.Temporal.RecordedAt)
	checkOptIso(c, joinPath(t, "reviewAt"), e.Temporal.ReviewAt)
	c.optStr(joinPath(t, "sessionRef"), e.Temporal.SessionRef, 1, 120)
	if e.Temporal.Sequence != nil && *e.Temporal.Sequence < 0 {
		c.add(joinPath(t, "sequence"), "Too small: expected number to be >=0")
	}

	q := joinPath(path, "quality")
	c.enum(joinPath(q, "directness"), e.Quality.Directness, []string{"direct", "indirect", "inferred"})
	c.enum(joinPath(q, "recency"), e.Quality.Recency, RecencyBands)
	c.enum(joinPath(q, "completeness"), e.Quality.Completeness, []string{"complete", "partial", "unknown"})
	c.enum(joinPath(q, "reliability"), e.Quality.Reliability, []string{"instrument-defined", "user-asserted", "unvalidated-method", "unknown"})
	c.enum(joinPath(q, "validity"), e.Quality.Validity, []string{"validated-instrument", "unvalidated-instrument", "no-instrument", "disputed"})
	c.intRange(joinPath(q, "sampleSize"), e.Quality.SampleSize, 1, 1_000_000, true)
	c.optStr(joinPath(q, "notes"), e.Quality.Notes, 0, 500)

	if e.Status == "primary" && len(e.Provenance.Lineage) > 0 {
		c.add(joinPath(path, "provenance", "lineage"), "primary evidence cannot have lineage; a derived record must justify its derivation")
	}
	if e.Status == "derived" && len(e.Provenance.Lineage) == 0 {
		c.add(joinPath(path, "provenance", "lineage"), "derived evidence must reference the evidence it was derived from")
	}
	if e.Status == "primary" && e.Provenance.SourceKind == "derived" {
		c.add(joinPath(path, "provenance", "sourceKind"), "a primary record cannot claim a derived source kind")
	}
}

func ValidateEvidenceRecord(e EvidenceRecord) []Issue {
	c := &checker{}
	checkEvidenceRecord(c, "", e)
	return c.result()
}

// EvidenceBundle is a traceable bundle: one primary observation plus any
// derived records. Promoted claims must resolve to a non-null evidence
// reference (§21).
type EvidenceBundle = []EvidenceRecord

func checkEvidenceBundle(c *checker, path string, records []EvidenceRecord) {
	if records == nil {
		c.add(path, "Invalid input: expected array, received undefined")
		return
	}
	c.arr(path, len(records), 1, 24)
	for i, record := range records {
		checkEvidenceRecord(c, idx(path, i), record)
	}
	primaries := 0
	ids := make(map[string]bool, len(records))
	for _, record := range records {
		if record.Status == "primary" {
			primaries++
		}
		ids[record.ID] = true
	}
	if primaries != 1 {
		c.add(path, fmt.Sprintf("an evidence bundle must contain exactly one primary record, found %d", primaries))
	}
	for _, record := range records {
		for _, parent := range record.Provenance.Lineage {
			if !ids[parent] {
				c.add(path, fmt.Sprintf("derived evidence %s references absent lineage %s", record.ID, parent))
			}
		}
	}
}

func ValidateEvidenceBundle(records []EvidenceRecord) []Issue {
	c := &checker{}
	checkEvidenceBundle(c, "", records)
	return c.result()
}

// PrimaryOf returns the bundle's primary record.
func PrimaryOf(bundle []EvidenceRecord) (EvidenceRecord, bool) {
	for _, record := range bundle {
		if record.Status == "primary" {
			return record, true
		}
	}
	return EvidenceRecord{}, false
}

// RefusalForMissingEvidence is the refusal contract (§21): any claim promoted
// out of the evidence layer must carry a non-null evidence reference.
func RefusalForMissingEvidence(subject string) string {
	return fmt.Sprintf("refusing to promote %q without an evidence reference: promoted claims require non-null evidence", subject)
}

// --------------------------------------------------------------- builders

type PrimaryEvidenceParams struct {
	SignalID          string
	SourceRef         string
	SourceKind        string
	AssertedBy        *ActorRef
	CapturedBy        ActorRef
	MethodRef         string
	MethodKind        string
	MethodVersion     string
	MethodDescription string
	Instrument        *InstrumentRef
	Limitations       []string
	Temporal          TemporalRecord
	Quality           EvidenceQuality
	SchemaVersion     string
}

// CreatePrimaryEvidence builds the primary evidence record for an observation.
// It rejects at the boundary: an anonymous observation (no source reference)
// is not evidence, and the system never invents provenance for it.
func CreatePrimaryEvidence(params PrimaryEvidenceParams) (EvidenceRecord, []string) {
	var issues []string
	if strings.TrimSpace(params.SourceRef) == "" {
		issues = append(issues, "provenance.sourceRef: primary evidence requires a stable source reference")
	}
	if strings.TrimSpace(params.MethodRef) == "" {
		issues = append(issues, "method.ref: primary evidence requires a measurement method reference")
	}
	if isoMillis(params.Temporal.RecordedAt) < isoMillis(params.Temporal.ObservedAt) {
		issues = append(issues, "temporal.recordedAt: an observation cannot be recorded before it occurred")
	}

	assertedBy := UserZeroActor
	if params.AssertedBy != nil {
		assertedBy = *params.AssertedBy
	}
	schemaVersion := params.SchemaVersion
	if schemaVersion == "" {
		schemaVersion = EvidenceSchemaVersion
	}
	candidate := EvidenceRecord{
		ID:            params.SignalID,
		SignalID:      params.SignalID,
		SchemaVersion: schemaVersion,
		Status:        "primary",
		Provenance: Provenance{
			SourceRef:  params.SourceRef,
			SourceKind: params.SourceKind,
			AssertedBy: assertedBy,
			CapturedBy: params.CapturedBy,
			Lineage:    []string{},
		},
		Method: MeasurementMethod{
			Ref:         params.MethodRef,
			Kind:        params.MethodKind,
			Version:     params.MethodVersion,
			Description: params.MethodDescription,
			Instrument:  params.Instrument,
			Limitations: CopyStrings(params.Limitations),
		},
		Temporal: params.Temporal,
		Quality:  params.Quality,
	}

	if schemaIssues := ValidateEvidenceRecord(candidate); len(schemaIssues) > 0 {
		return EvidenceRecord{}, append(issues, IssueStrings(schemaIssues)...)
	}
	if len(issues) > 0 {
		return EvidenceRecord{}, issues
	}
	return candidate, nil
}

type DerivedEvidenceParams struct {
	SignalID          string
	ParentEvidenceIDs []string
	SourceRef         string
	CapturedBy        ActorRef
	MethodRef         string
	MethodKind        string
	MethodVersion     string
	MethodDescription string
	Limitations       []string
	Temporal          TemporalRecord
	Quality           EvidenceQuality
	// Distinguishes several derived records within the same bundle.
	Discriminator string
	SchemaVersion string
}

// CreateDerivedEvidence builds a derived evidence record; lineage is mandatory
// and recorded.
func CreateDerivedEvidence(params DerivedEvidenceParams) (EvidenceRecord, []string) {
	var issues []string
	if len(params.ParentEvidenceIDs) == 0 {
		issues = append(issues, "provenance.lineage: derived evidence must reference its parents")
	}
	parents := CopyStrings(params.ParentEvidenceIDs)
	sort.Strings(parents)
	id := MintID("evd", map[string]any{
		"signalId":      params.SignalID,
		"parents":       parents,
		"discriminator": params.Discriminator,
	})
	schemaVersion := params.SchemaVersion
	if schemaVersion == "" {
		schemaVersion = EvidenceSchemaVersion
	}
	candidate := EvidenceRecord{
		ID:            id,
		SignalID:      params.SignalID,
		SchemaVersion: schemaVersion,
		Status:        "derived",
		Provenance: Provenance{
			SourceRef:  params.SourceRef,
			SourceKind: "derived",
			AssertedBy: params.CapturedBy,
			CapturedBy: params.CapturedBy,
			Lineage:    CopyStrings(params.ParentEvidenceIDs),
		},
		Method: MeasurementMethod{
			Ref:         params.MethodRef,
			Kind:        params.MethodKind,
			Version:     params.MethodVersion,
			Description: params.MethodDescription,
			Limitations: CopyStrings(params.Limitations),
		},
		Temporal: params.Temporal,
		Quality:  params.Quality,
	}
	if schemaIssues := ValidateEvidenceRecord(candidate); len(schemaIssues) > 0 {
		return EvidenceRecord{}, append(issues, IssueStrings(schemaIssues)...)
	}
	if len(issues) > 0 {
		return EvidenceRecord{}, issues
	}
	return candidate, nil
}

func CreateEvidenceBundle(records []EvidenceRecord) ([]EvidenceRecord, []string) {
	bundle := append([]EvidenceRecord{}, records...)
	if issues := ValidateEvidenceBundle(bundle); len(issues) > 0 {
		return nil, IssueStrings(issues)
	}
	return bundle, nil
}

type QualityParams struct {
	Instrument             *InstrumentRef
	ResponseComplete       bool
	ObservedAt             string
	Now                    string
	HasIndependentEvidence bool
	SampleSize             int
	Notes                  string
}

// QualityFor derives quality metadata from what is actually recorded about
// the method.
func QualityFor(params QualityParams) EvidenceQuality {
	validated := params.Instrument != nil && params.Instrument.Validated
	ageDays := float64(isoMillis(params.Now)-isoMillis(params.ObservedAt)) / 86_400_000
	recency := "stale"
	switch {
	case ageDays < 0:
		recency = "unknown"
	case ageDays <= 180:
		recency = "current"
	case ageDays <= 730:
		recency = "recent"
	}
	completeness := "unknown"
	if params.ResponseComplete {
		completeness = "partial"
		if params.HasIndependentEvidence {
			completeness = "complete"
		}
	}
	reliability, validity := "unknown", "no-instrument"
	if params.Instrument != nil {
		if validated {
			reliability, validity = "instrument-defined", "validated-instrument"
		} else {
			reliability, validity = "user-asserted", "unvalidated-instrument"
		}
	}
	sampleSize := params.SampleSize
	if sampleSize == 0 {
		sampleSize = 1
	}
	return EvidenceQuality{
		Directness:   "direct",
		Recency:      recency,
		Completeness: completeness,
		Reliability:  reliability,
		Validity:     validity,
		SampleSize:   sampleSize,
		Notes:        params.Notes,
	}
}
