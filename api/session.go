// Package api serves the Career Assessment 107 runtime over HTTP for the web
// and mobile projections, together with the bundled web UI.
//
// The UI never owns career state: it asks this API for domain state and
// renders it (UI ≠ Truth, §14). Every mutation goes through the runtime, so
// the same validation, authorization, and invariants apply as anywhere else.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"career107/domain"
	"career107/engine"
)

const (
	SubjectID      = "sub_web-session"
	SubjectLabel   = "This session (subject sub_web-session)"
	SessionRuntime = "career107-runtime.web"
	journalVersion = 1
)

// journalClock records every clock reading taken while an action runs, and
// replays recorded readings when the journal is loaded. Because identities and
// digests are derived from timestamps, replaying the same actions with the
// same readings rebuilds exactly the same state.
type journalClock struct {
	mu        sync.Mutex
	replay    []string
	recording bool
	recorded  []string
}

func (c *journalClock) Now() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.replay) > 0 {
		value := c.replay[0]
		c.replay = c.replay[1:]
		return value
	}
	value := domain.FormatIso(time.Now())
	if c.recording {
		c.recorded = append(c.recorded, value)
	}
	return value
}

func (c *journalClock) begin() {
	c.mu.Lock()
	c.recording = true
	c.recorded = nil
	c.mu.Unlock()
}

func (c *journalClock) end() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recording = false
	out := c.recorded
	c.recorded = nil
	return out
}

type journalEntry struct {
	Op       string          `json:"op"`
	Args     json.RawMessage `json:"args"`
	Readings []string        `json:"readings"`
}

type journalFile struct {
	Version  int            `json:"version"`
	Created  []string       `json:"created"`
	Entries  []journalEntry `json:"entries"`
	Warnings []string       `json:"-"`
}

// Session is one user's session: a runtime plus an optional on-disk journal.
// It is safe for concurrent use; actions are applied one at a time.
type Session struct {
	mu      sync.Mutex
	dataDir string
	clock   *journalClock
	runtime *engine.CareerRuntime
	journal journalFile
}

type captureArgs struct {
	Input domain.SignalInput `json:"input"`
}

type decomposeArgs struct {
	SignalID string `json:"signalId"`
}

type reviewArgs struct {
	PathwayID string `json:"pathwayId"`
	State     string `json:"state"`
	Rationale string `json:"rationale,omitempty"`
}

type correctArgs struct {
	RepresentationID string `json:"representationId"`
	CorrectedSummary string `json:"correctedSummary"`
}

type proposeArgs struct {
	RepresentationID string `json:"representationId"`
}

// NewSession creates a session. When dataDir is not empty, the session is
// restored from and saved to dataDir/session.json.
func NewSession(dataDir string) (*Session, error) {
	s := &Session{dataDir: dataDir}
	if err := s.restore(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Session) journalPath() string {
	return filepath.Join(s.dataDir, "session.json")
}

func newRuntime(clock domain.Clock) *engine.CareerRuntime {
	return engine.New(engine.Options{
		Clock:     clock,
		RuntimeID: SessionRuntime,
		// A user session is long-lived; the development default budget (64
		// invocations per capability) is meant for tests and would lock the
		// app after a few dozen answer changes.
		GrantBudget: 1_000_000,
		Model:       &engine.DeterministicStubModelProvider{Uncertainty: "Placeholder provider: the proposal text is a contract demonstration, not a model judgement."},
	})
}

func (s *Session) fresh() {
	s.clock = &journalClock{}
	s.clock.begin()
	s.runtime = newRuntime(s.clock)
	s.journal = journalFile{Version: journalVersion, Created: s.clock.end(), Entries: []journalEntry{}}
}

// restore replays the journal, or starts empty when there is none. A journal
// that cannot be replayed is set aside rather than deleted.
func (s *Session) restore() error {
	if s.dataDir == "" {
		s.fresh()
		return nil
	}
	raw, err := os.ReadFile(s.journalPath())
	if errors.Is(err, os.ErrNotExist) {
		s.fresh()
		return nil
	}
	if err != nil {
		return err
	}
	var stored journalFile
	if err := json.Unmarshal(raw, &stored); err != nil || stored.Version != journalVersion {
		s.setAside()
		s.fresh()
		return nil
	}
	s.clock = &journalClock{replay: append([]string{}, stored.Created...)}
	s.runtime = newRuntime(s.clock)
	for _, entry := range stored.Entries {
		s.clock.replay = append(s.clock.replay, entry.Readings...)
		if _, _, err := s.apply(entry.Op, entry.Args); err != nil {
			s.setAside()
			s.fresh()
			return nil
		}
		if len(s.clock.replay) != 0 {
			// The journal no longer matches the code that replays it.
			s.setAside()
			s.fresh()
			return nil
		}
	}
	s.journal = stored
	return nil
}

func (s *Session) setAside() {
	_ = os.Rename(s.journalPath(), s.journalPath()+fmt.Sprintf(".unreadable-%d", time.Now().Unix()))
}

func (s *Session) save() error {
	if s.dataDir == "" {
		return nil
	}
	if err := os.MkdirAll(s.dataDir, 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(s.journal)
	if err != nil {
		return err
	}
	tmp := s.journalPath() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.journalPath())
}

// Do runs one action, records it in the journal, and returns the notice and
// any rejections for the UI.
func (s *Session) Do(op string, args any) (string, []string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := json.Marshal(args)
	if err != nil {
		return "", nil, err
	}
	s.clock.begin()
	notice, rejections, err := s.apply(op, raw)
	readings := s.clock.end()
	if err != nil {
		return "", nil, err
	}
	if len(readings) > 0 {
		s.journal.Entries = append(s.journal.Entries, journalEntry{Op: op, Args: raw, Readings: readings})
		if err := s.save(); err != nil {
			return notice, rejections, fmt.Errorf("saving session: %w", err)
		}
	}
	return notice, rejections, nil
}

// Reset discards the session and its journal.
func (s *Session) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fresh()
	if s.dataDir == "" {
		return nil
	}
	if err := os.Remove(s.journalPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func describe(rejections []domain.Rejection) []string {
	out := make([]string, len(rejections))
	for i, r := range rejections {
		out[i] = r.Code + ": " + r.Message
	}
	return out
}

func (s *Session) latestSet() (domain.PathwaySet, bool) {
	sets := s.runtime.ListPathwaySets(SubjectID)
	if len(sets) == 0 {
		return domain.PathwaySet{}, false
	}
	return sets[len(sets)-1], true
}

// apply performs one action against the runtime. It returns an error only for
// malformed requests; domain refusals are returned as rejections.
func (s *Session) apply(op string, raw json.RawMessage) (string, []string, error) {
	r := s.runtime
	switch op {
	case "capture":
		var args captureArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", nil, fmt.Errorf("capture: %w", err)
		}
		out, rejections := r.CaptureSignal(args.Input, engine.CaptureOptions{})
		if len(rejections) > 0 {
			return "", describe(rejections), nil
		}
		return fmt.Sprintf("S0 captured: signal %s · %d evidence record(s) · %d uncertainty note(s). Next: decompose it into (A, E, C) | U.",
			out.Signal.ID, len(out.Signal.Evidence), len(out.Signal.Uncertainty.Notes)), nil, nil

	case "answer":
		// One step for the app: capture the answer (S0) and decompose it into
		// (A, E, C) | U (S1→S4). Each step still runs through its own boundary.
		var args captureArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", nil, fmt.Errorf("answer: %w", err)
		}
		captured, rejections := r.CaptureSignal(args.Input, engine.CaptureOptions{})
		if len(rejections) > 0 {
			return "", describe(rejections), nil
		}
		out, rejections := r.DecomposeSignal(captured.Signal.ID, engine.InvokeOptions{})
		if len(rejections) > 0 {
			return "", describe(rejections), nil
		}
		return fmt.Sprintf("Answer recorded as observation %s and representation %s.", captured.Signal.ID, out.Representation.ID), nil, nil

	case "decompose":
		var args decomposeArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", nil, fmt.Errorf("decompose: %w", err)
		}
		out, rejections := r.DecomposeSignal(args.SignalID, engine.InvokeOptions{})
		if len(rejections) > 0 {
			return "", describe(rejections), nil
		}
		return fmt.Sprintf("S1 → S4: capability %s · transformation %s@%s · representation %s · artifacts %d.",
			out.Execution.CapabilityRef, out.Transformation.TransformationID, out.Transformation.TransformationVersion,
			out.Representation.ID, len(out.Execution.ArtifactRefs)), nil, nil

	case "map":
		// A changed answer is a new observation that supersedes the earlier
		// one: the earlier one stays in the history, but only the latest
		// answer to each question is mapped.
		ids := []string{}
		position := map[string]int{}
		for _, representation := range r.ListRepresentations(SubjectID) {
			item := representation.A.Instrument.ItemRef
			if i, seen := position[item]; seen {
				ids[i] = representation.ID
				continue
			}
			position[item] = len(ids)
			ids = append(ids, representation.ID)
		}
		out, rejections := r.MapPathways(ids, engine.InvokeOptions{})
		if len(rejections) > 0 {
			return "", describe(rejections), nil
		}
		set := out.PathwaySet
		return fmt.Sprintf("Mapped %d representation(s) under %s; %d pathway candidates carry %d unassessed construct(s). Deterministic mapping, declared losses recorded.",
			set.Coverage.RepresentationCount, set.MappingPolicyRef, len(set.Pathways), len(set.Coverage.UnassessedConstructs)), nil, nil

	case "review":
		var args reviewArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", nil, fmt.Errorf("review: %w", err)
		}
		set, ok := s.latestSet()
		if !ok {
			return "", []string{"S4.PATHWAY-SET.NOT-FOUND: map pathways before reviewing one"}, nil
		}
		if !domain.Contains([]string{"interesting", "not-a-fit", "pursuing", "deferred"}, args.State) {
			return "", []string{"S4.PATHWAY.REVIEW-STATE: review state must be interesting, not-a-fit, pursuing, or deferred"}, nil
		}
		out, rejections := r.SetPathwayReview(engine.ReviewParams{
			PathwaySetID: set.ID,
			PathwayID:    args.PathwayID,
			State:        args.State,
			DecidedBy:    domain.UserZeroActor,
			Rationale:    args.Rationale,
		})
		if len(rejections) > 0 {
			return "", describe(rejections), nil
		}
		return fmt.Sprintf("Decision recorded for %s: %s (by user 0, %s).", args.PathwayID, args.State, out.Decision.DecidedAt), nil, nil

	case "correct":
		var args correctArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", nil, fmt.Errorf("correct: %w", err)
		}
		representation, ok := r.GetRepresentation(args.RepresentationID)
		if !ok {
			return "", []string{fmt.Sprintf("S1.REPRESENTATION.NOT-FOUND: no representation %q", args.RepresentationID)}, nil
		}
		previous := make([]string, len(representation.C.Fields))
		for i, field := range representation.C.Fields {
			previous[i] = field.Key + "=" + field.Value.Join(",")
		}
		correction, rejections := r.CorrectRepresentation(engine.CorrectionParams{
			RepresentationID:      args.RepresentationID,
			Dimension:             "C",
			Field:                 "context.note",
			PreviousValueSummary:  strings.Join(previous, "; "),
			CorrectedValueSummary: args.CorrectedSummary,
			Reason:                "corrected by the user in the app",
			CorrectedBy:           domain.UserZeroActor,
		})
		if len(rejections) > 0 {
			return "", describe(rejections), nil
		}
		return fmt.Sprintf("Correction %s recorded against %s. The original representation is untouched: corrections are new records, not edits.",
			correction.ID, args.RepresentationID), nil, nil

	case "propose":
		var args proposeArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", nil, fmt.Errorf("propose: %w", err)
		}
		out, rejections := r.ProposeQuestions(args.RepresentationID, engine.InvokeOptions{})
		if len(rejections) > 0 {
			return "", describe(rejections), nil
		}
		m := out.Proposal.ModelMetadata
		notice := fmt.Sprintf("Proposal %s recorded as %s by %s/%s@%s. Proposals are not evidence and require your decision.",
			out.Proposal.ID, out.Proposal.Status, m.Provider, m.Model, m.ModelVersion)
		if len(out.Findings) > 0 {
			codes := make([]string, len(out.Findings))
			for i, finding := range out.Findings {
				codes[i] = finding.Code
			}
			notice += " Findings: " + strings.Join(codes, ", ") + "."
		}
		return notice, nil, nil
	}
	return "", nil, fmt.Errorf("unknown action %q", op)
}

// ---------------------------------------------------------------- view

type SessionView struct {
	Projection        engine.RuntimeProjection                 `json:"projection"`
	Signals           []domain.Signal                          `json:"signals"`
	Representations   []domain.Representation1to3              `json:"representations"`
	Rendered          map[string]domain.RenderedRepresentation `json:"rendered"`
	PathwaySets       []domain.PathwaySet                      `json:"pathwaySets"`
	DecisionSummaries map[string]domain.DecisionSummary        `json:"decisionSummaries"`
	Decisions         []domain.UserDecisionRecord              `json:"decisions"`
	Corrections       []domain.CorrectionRecord                `json:"corrections"`
	Proposals         []domain.ProposalRecord                  `json:"proposals"`
	Persisted         bool                                     `json:"persisted"`
}

// View collects the session's domain state for the UI. It is derived state:
// the UI renders it, but it is not the domain model.
func (s *Session) View() SessionView {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.runtime
	representations := r.ListRepresentations(SubjectID)
	rendered := map[string]domain.RenderedRepresentation{}
	for _, representation := range representations {
		rendered[representation.ID] = domain.RenderRepresentation(representation, domain.RenderOptions{})
	}
	sets := r.ListPathwaySets(SubjectID)
	summaries := map[string]domain.DecisionSummary{}
	for _, set := range sets {
		summaries[set.ID] = domain.UserDecisionSummary(set)
	}
	// Reading the projection takes a clock reading that is not part of any
	// action, so it is never journalled.
	return SessionView{
		Projection:        r.Projection(SubjectID),
		Signals:           r.ListSignals(SubjectID),
		Representations:   representations,
		Rendered:          rendered,
		PathwaySets:       sets,
		DecisionSummaries: summaries,
		Decisions:         r.ListDecisions(),
		Corrections:       r.ListCorrections(),
		Proposals:         r.ListProposals(),
		Persisted:         s.dataDir != "",
	}
}
