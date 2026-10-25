package domain

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Stable identities for the 1:3 infrastructure representation.
//
// Invariant (107 §6): SignalId(A_i) = SignalId(E_i) = SignalId(C_i).
// Identity is therefore minted once, at the signal (observation) level, and is
// carried — never re-minted — by the assessment, evidence, and context records
// that the signal decomposes into.
//
// Identifiers are deterministic: they are derived from canonical content plus
// a caller-supplied scope, so the same input yields the same identity and a
// result remains reproducible from input + versions + references.

const idBody = `[a-z0-9][a-z0-9-]{1,79}`

type idKind struct {
	prefix string
	name   string
	re     *regexp.Regexp
}

func newIDKind(prefix, name string) idKind {
	return idKind{prefix: prefix, name: name, re: regexp.MustCompile(`^` + prefix + `_` + idBody + `$`)}
}

func (k idKind) check(c *checker, path, value string) bool {
	return c.match(path, value, k.re, fmt.Sprintf("%s must match ^%s_<slug> (lowercase slug, 2-80 chars)", k.name, k.prefix))
}

// Valid reports whether value is a well-formed identifier of this kind.
func (k idKind) Valid(value string) bool { return k.re.MatchString(value) }

var (
	SubjectIDKind        = newIDKind("sub", "SubjectId")
	SignalIDKind         = newIDKind("sig", "SignalId")
	RepresentationIDKind = newIDKind("r13", "RepresentationId")
	TransformationIDKind = newIDKind("trf", "TransformationId")
	PathwayIDKind        = newIDKind("pth", "PathwayId")
	PathwaySetIDKind     = newIDKind("pths", "PathwaySetId")
	RecordIDKind         = newIDKind("rec", "RecordId")
	RevisionIDKind       = newIDKind("rev", "RevisionId")
	InputIDKind          = newIDKind("inp", "InputId")
	PlanIDKind           = newIDKind("plan", "PlanId")
	PlatformRepIDKind    = newIDKind("plat", "PlatformRepId")
	ExecutionIDKind      = newIDKind("exe", "ExecutionId")
	ArtifactIDKind       = newIDKind("art", "ArtifactId")
	ProposalIDKind       = newIDKind("prp", "ProposalId")
	SessionIDKind        = newIDKind("ses", "SessionId")
	GrantIDKind          = newIDKind("grn", "GrantId")
)

// Capability identifiers are dotted namespaces owned by 107, never free-form.
var capabilityIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+$`)

const capabilityIDMessage = "capability id must be a dotted lowercase namespace, e.g. career.assessment.decompose-1to3"

func checkCapabilityID(c *checker, path, value string) bool {
	return c.match(path, value, capabilityIDPattern, capabilityIDMessage)
}

var semverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

func checkSemver(c *checker, path, value string) bool {
	return c.match(path, value, semverPattern, "semantic version must be MAJOR.MINOR.PATCH")
}

var isoPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d{1,3})?Z$`)

func checkIso(c *checker, path, value string) bool {
	return c.match(path, value, isoPattern, "timestamps must be ISO-8601 UTC with Z suffix")
}

func checkOptIso(c *checker, path, value string) {
	if value != "" {
		checkIso(c, path, value)
	}
}

// IsIsoTimestamp reports whether value is an ISO-8601 UTC timestamp with Z.
func IsIsoTimestamp(value string) bool { return isoPattern.MatchString(value) }

// MintID mints a deterministic identifier.
//
// prefix is the id namespace; scope is the canonical content + scope (e.g.
// subject, signal, moment) that makes the identity unique and reproducible.
func MintID(prefix string, scope any) string {
	digest := DigestOf(FNV1a64Hasher, Canonicalize(scope), prefix+":identity")
	value := digest.Value
	if len(value) > 20 {
		value = value[:20]
	}
	return prefix + "_" + value
}

// ------------------------------------------------------------------- clocks

const isoLayout = "2006-01-02T15:04:05.000Z"

// FormatIso renders a time as ISO-8601 UTC with millisecond precision.
func FormatIso(t time.Time) string {
	return t.UTC().Format(isoLayout)
}

// ParseIso parses an ISO-8601 UTC timestamp. Invalid input yields the zero time.
func ParseIso(value string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

func isoMillis(value string) int64 {
	t, ok := ParseIso(value)
	if !ok {
		return 0
	}
	return t.UnixMilli()
}

// Clock is injected: reproducibility requires that time is not read implicitly.
type Clock interface {
	Now() string
}

type systemClock struct{}

func (systemClock) Now() string { return FormatIso(time.Now()) }

// SystemClock reads wall-clock time.
var SystemClock Clock = systemClock{}

type fixedClock struct{ iso string }

func (c fixedClock) Now() string { return c.iso }

func FixedClock(iso string) Clock {
	if !IsIsoTimestamp(iso) {
		panic("fixedClock: timestamps must be ISO-8601 UTC with Z suffix")
	}
	return fixedClock{iso: iso}
}

type steppingClock struct {
	mu      sync.Mutex
	current time.Time
	step    time.Duration
}

func (c *steppingClock) Now() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	value := FormatIso(c.current)
	c.current = c.current.Add(c.step)
	return value
}

// SteppingClock is the monotonic logical clock used by deterministic
// experiments and tests: every reading advances it by stepMs.
func SteppingClock(startIso string, stepMs int64) Clock {
	start, ok := ParseIso(startIso)
	if !ok || !IsIsoTimestamp(startIso) {
		panic("steppingClock: timestamps must be ISO-8601 UTC with Z suffix")
	}
	return &steppingClock{current: start, step: time.Duration(stepMs) * time.Millisecond}
}

func IsoTimestampAfter(iso string, deltaMs int64) string {
	t, _ := ParseIso(iso)
	return FormatIso(t.Add(time.Duration(deltaMs) * time.Millisecond))
}

func CompareIso(a, b string) int {
	return strings.Compare(a, b)
}

// ------------------------------------------------------------------- actors
//
// The 0 boundary expressed as data. The user is 0: the origin of intent and
// the decision authority. The system is 1:3 infrastructure. Every record
// states which actor produced it, and no system actor may claim the user's
// authority.

const (
	ActorUser       = "user"
	ActorSystem     = "system"
	ActorCapability = "capability"
	ActorModel      = "model"
)

var ActorKinds = []string{ActorUser, ActorSystem, ActorCapability, ActorModel}

type ActorRef struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
}

func checkActor(c *checker, path string, actor ActorRef) {
	c.enum(joinPath(path, "kind"), actor.Kind, ActorKinds)
	c.str(joinPath(path, "id"), actor.ID, 1, -1)
}

// UserZeroActor is the reserved identity of user 0 — the origin and decision
// authority.
var UserZeroActor = ActorRef{Kind: ActorUser, ID: "usr_0", Label: "User 0 (origin and decision authority)"}

var SystemActor = ActorRef{Kind: ActorSystem, ID: "sys_107", Label: "Career Assessment 107 infrastructure"}

var Cloud107Actor = ActorRef{Kind: ActorSystem, ID: "sys_cloud107", Label: "Cloud107 runtime (infrastructure only)"}

func CapabilityActor(capabilityID string) ActorRef {
	return ActorRef{Kind: ActorCapability, ID: "cap_" + capabilityID, Label: capabilityID}
}

var nonModelIDChars = regexp.MustCompile(`(?i)[^a-z0-9-]`)

func ModelActor(provider, model string) ActorRef {
	id := strings.ToLower(nonModelIDChars.ReplaceAllString("mod_"+provider+"-"+model, "-"))
	return ActorRef{Kind: ActorModel, ID: id, Label: provider + "/" + model}
}

func IsUserActor(actor ActorRef) bool {
	return actor.Kind == ActorUser
}

// Subject is who an assessment is about. It is not necessarily the decision
// authority: a third-party subject requires an explicit consent reference.
type Subject struct {
	ID            string    `json:"id"`
	Kind          string    `json:"kind"`
	LinkedUserRef *ActorRef `json:"linkedUserRef,omitempty"`
	ConsentRef    string    `json:"consentRef,omitempty"`
	Notes         string    `json:"notes,omitempty"`
}

func SelfSubject(id string) Subject {
	user := UserZeroActor
	return Subject{ID: id, Kind: "self", LinkedUserRef: &user}
}
