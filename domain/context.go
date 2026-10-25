package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

// C of the 1:3 representation.
//
// C answers: in which context may this observation legitimately be relevant?
// Context is descriptive, never evaluative: it explains the conditions under
// which an observation was produced, and it is what makes the
// context-dependence experiment (research experiment C) possible: hold A
// constant, change C, and legitimate interpretation must be allowed to change.
//
// Identity note: the context record carries the signal identity.

const ContextSchemaVersion = "1.0.0"

var ContextKinds = []string{
	"declared",      // the user stated it (setting, constraints, preferences)
	"session",       // properties of the assessment sitting itself
	"instrument",    // properties of the instrument administration
	"environmental", // labour-market / organisational setting at observation time
	"mixed",
}

// ContextValue is a context field value: a string, a finite number, a
// boolean, or a list of strings.
type ContextValue struct {
	v any
}

func StringValue(s string) ContextValue     { return ContextValue{v: s} }
func NumberValue(f float64) ContextValue    { return ContextValue{v: f} }
func BoolValue(b bool) ContextValue         { return ContextValue{v: b} }
func ListValue(items []string) ContextValue { return ContextValue{v: CopyStrings(items)} }

// Raw returns the underlying value (string, float64, bool, []string or nil).
func (c ContextValue) Raw() any { return c.v }

func (c ContextValue) AsString() (string, bool) {
	s, ok := c.v.(string)
	return s, ok
}

func (c ContextValue) AsList() ([]string, bool) {
	list, ok := c.v.([]string)
	return list, ok
}

// String renders the value as text: lists joined by "|", everything else as
// JavaScript's String(value) would.
func (c ContextValue) String() string {
	return c.Join("|")
}

// Join renders a list with sep and scalars as text.
func (c ContextValue) Join(sep string) string {
	switch v := c.v.(type) {
	case string:
		return v
	case float64:
		return JSNumber(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case []string:
		return strings.Join(v, sep)
	}
	return "undefined"
}

func (c ContextValue) MarshalJSON() ([]byte, error) {
	if c.v == nil {
		return []byte("null"), nil
	}
	return json.Marshal(c.v)
}

func (c *ContextValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		c.v = nil
		return nil
	}
	switch trimmed[0] {
	case '"':
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return err
		}
		c.v = s
	case '[':
		var list []string
		if err := json.Unmarshal(trimmed, &list); err != nil {
			return fmt.Errorf("context value list must contain strings: %w", err)
		}
		if list == nil {
			list = []string{}
		}
		c.v = list
	case 't', 'f':
		var b bool
		if err := json.Unmarshal(trimmed, &b); err != nil {
			return err
		}
		c.v = b
	default:
		var f float64
		if err := json.Unmarshal(trimmed, &f); err != nil {
			return fmt.Errorf("context value must be a string, number, boolean, or string list")
		}
		c.v = f
	}
	return nil
}

func checkContextValue(c *checker, path string, value ContextValue) {
	switch v := value.v.(type) {
	case string:
		c.str(path, v, 1, 160)
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			c.add(path, "Invalid input: expected number, received NaN")
		}
	case bool:
	case []string:
		c.strings(path, v, 1, 160, 1, 12)
	default:
		c.add(path, "Invalid input: expected string, number, boolean, or string list")
	}
}

type ContextField struct {
	Key   string       `json:"key"`
	Value ContextValue `json:"value"`
	// Who asserted the field. Context asserted by the system must be marked as
	// such so that it can never be mistaken for a user statement.
	AssertedBy ActorRef `json:"assertedBy"`
	// Reference supporting the assertion (e.g. an evidence id) when applicable.
	SupportRef string `json:"supportRef,omitempty"`
}

type CareerContext struct {
	ID            string         `json:"id"`
	SchemaVersion string         `json:"schemaVersion"`
	Kind          string         `json:"kind"`
	Fields        []ContextField `json:"fields"`
}

func checkContext(c *checker, path string, ctx CareerContext) {
	SignalIDKind.check(c, joinPath(path, "id"), ctx.ID)
	checkSemver(c, joinPath(path, "schemaVersion"), ctx.SchemaVersion)
	c.enum(joinPath(path, "kind"), ctx.Kind, ContextKinds)
	fieldsPath := joinPath(path, "fields")
	c.arr(fieldsPath, len(ctx.Fields), 1, 24)
	seen := map[string]bool{}
	for i, field := range ctx.Fields {
		fieldPath := idx(fieldsPath, i)
		checkContextFieldKey(c, joinPath(fieldPath, "key"), field.Key)
		checkContextValue(c, joinPath(fieldPath, "value"), field.Value)
		checkActor(c, joinPath(fieldPath, "assertedBy"), field.AssertedBy)
		c.optStr(joinPath(fieldPath, "supportRef"), field.SupportRef, 1, 120)
	}
	for i, field := range ctx.Fields {
		key := strings.ToLower(field.Key)
		if seen[key] {
			c.add(joinPath(idx(fieldsPath, i), "key"), fmt.Sprintf("duplicate context field key %q", field.Key))
		}
		seen[key] = true
	}
}

func ValidateContext(ctx CareerContext) []Issue {
	c := &checker{}
	checkContext(c, "", ctx)
	return c.result()
}

// RequiredContextKeys is the minimum context for a career-relevant
// observation. Career mapping is explicitly context-dependent (§7).
var RequiredContextKeys = []string{"setting", "careerStage"}

func FindContextField(ctx CareerContext, key string) (ContextField, bool) {
	needle := strings.ToLower(key)
	for _, field := range ctx.Fields {
		if strings.ToLower(field.Key) == needle {
			return field, true
		}
	}
	return ContextField{}, false
}

func ContextSignature(ctx CareerContext) string {
	parts := make([]string, len(ctx.Fields))
	for i, field := range ctx.Fields {
		parts[i] = field.Key + "=" + field.Value.String()
	}
	sort.Strings(parts)
	return strings.Join(parts, ";")
}

// --------------------------------------------------------------- builders

type ContextFieldInput struct {
	Key        string
	Value      ContextValue
	AssertedBy *ActorRef
	SupportRef string
}

type CreateContextParams struct {
	// Must be the identity of the originating observation (invariant §6).
	ID            string
	Kind          string
	Fields        []ContextFieldInput
	SchemaVersion string
}

// CreateContext builds a context record bound to an observation identity.
// It rejects prohibited field keys and empty field lists before anything
// downstream can treat "no context" as "no constraints" (§10, §17).
func CreateContext(params CreateContextParams) (CareerContext, []string) {
	var issues []string
	if len(params.Fields) == 0 {
		issues = append(issues, `context must declare at least one field; use an explicit "unassessed" marker instead of an empty context`)
	}
	schemaVersion := params.SchemaVersion
	if schemaVersion == "" {
		schemaVersion = ContextSchemaVersion
	}
	fields := make([]ContextField, len(params.Fields))
	for i, field := range params.Fields {
		assertedBy := UserZeroActor
		if field.AssertedBy != nil {
			assertedBy = *field.AssertedBy
		}
		value := field.Value
		if list, ok := value.AsList(); ok {
			value = ListValue(list)
		}
		fields[i] = ContextField{Key: field.Key, Value: value, AssertedBy: assertedBy, SupportRef: field.SupportRef}
	}
	candidate := CareerContext{ID: params.ID, SchemaVersion: schemaVersion, Kind: params.Kind, Fields: fields}
	if schemaIssues := ValidateContext(candidate); len(schemaIssues) > 0 {
		return CareerContext{}, append(issues, IssueStrings(schemaIssues)...)
	}
	if len(issues) > 0 {
		return CareerContext{}, issues
	}
	return candidate, nil
}

// ContextDelta lists context differences, used by the context-dependence
// experiment (research C).
type ContextDelta struct {
	Added     []string `json:"added"`
	Removed   []string `json:"removed"`
	Changed   []string `json:"changed"`
	Identical bool     `json:"identical"`
}

func DiffContexts(a, b CareerContext) ContextDelta {
	index := func(ctx CareerContext) (map[string]string, []string) {
		values := map[string]string{}
		var order []string
		for _, field := range ctx.Fields {
			key := strings.ToLower(field.Key)
			if _, exists := values[key]; !exists {
				order = append(order, key)
			}
			values[key] = field.Value.String()
		}
		return values, order
	}
	left, leftOrder := index(a)
	right, rightOrder := index(b)
	added, removed, changed := []string{}, []string{}, []string{}
	for _, key := range rightOrder {
		if _, ok := left[key]; !ok {
			added = append(added, key)
		}
	}
	for _, key := range leftOrder {
		if _, ok := right[key]; !ok {
			removed = append(removed, key)
		}
	}
	for _, key := range leftOrder {
		if other, ok := right[key]; ok && other != left[key] {
			changed = append(changed, key)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(changed)
	return ContextDelta{
		Added:     added,
		Removed:   removed,
		Changed:   changed,
		Identical: len(added) == 0 && len(removed) == 0 && len(changed) == 0,
	}
}

func ContextFieldMap(ctx CareerContext) map[string]ContextField {
	out := map[string]ContextField{}
	for _, field := range ctx.Fields {
		out[strings.ToLower(field.Key)] = field
	}
	return out
}
