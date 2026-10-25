// Package domain is the Career Assessment 107 bounded domain.
//
//	0_User → 1:3_Infrastructure → Information → 0_User
//
// It owns career semantics only: representation, constraint, evidence,
// context, uncertainty, transformation, pathway, invariants, versioning and
// records. It has no dependency on infrastructure (no execution, no
// scheduling, no persistence technology, no UI) — that is Cloud107's side of
// the boundary (§8).
//
// Rejections are values, never panics used for control flow: invalid
// representations are rejected before transformation or execution (§10).
package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Rejection is a structured refusal with a stable code so the UI, the runtime,
// and the research experiments can measure it.
type Rejection struct {
	// Stable machine code, e.g. "S1.IDENTITY.VIOLATION".
	Code string `json:"code"`
	// Human-readable explanation, written for a reviewer, not a machine.
	Message string `json:"message"`
	// Path into the rejected representation, when known.
	Path string `json:"path,omitempty"`
}

func NewRejection(code, message, path string) Rejection {
	return Rejection{Code: code, Message: message, Path: path}
}

func DescribeRejections(rejections []Rejection) string {
	lines := make([]string, 0, len(rejections))
	for _, item := range rejections {
		if item.Path == "" {
			lines = append(lines, fmt.Sprintf("%s: %s", item.Code, item.Message))
		} else {
			lines = append(lines, fmt.Sprintf("%s (%s): %s", item.Code, item.Path, item.Message))
		}
	}
	return strings.Join(lines, "\n")
}

// ValidationReport records how many checks ran, so "valid" is not confused
// with "not checked".
type ValidationReport struct {
	Valid      bool        `json:"valid"`
	Rejections []Rejection `json:"rejections"`
	ChecksRun  int         `json:"checksRun"`
}

func Report(checksRun int, rejections []Rejection) ValidationReport {
	if rejections == nil {
		rejections = []Rejection{}
	}
	return ValidationReport{Valid: len(rejections) == 0, Rejections: rejections, ChecksRun: checksRun}
}

// Issue is one schema violation found while validating a value at a boundary.
// It is the Go counterpart of a schema issue: a dotted path and a message.
type Issue struct {
	Path    string
	Message string
}

// Issues converts schema issues into rejections with a fixed code. An empty
// path is replaced by fallback, mirroring `issue.path.join('.') || fallback`.
func IssuesToRejections(code string, issues []Issue, fallback string) []Rejection {
	out := make([]Rejection, 0, len(issues))
	for _, issue := range issues {
		path := issue.Path
		if path == "" {
			path = fallback
		}
		out = append(out, NewRejection(code, issue.Message, path))
	}
	return out
}

// IssueStrings renders issues as "path: message" lines.
func IssueStrings(issues []Issue) []string {
	out := make([]string, 0, len(issues))
	for _, issue := range issues {
		path := issue.Path
		if path == "" {
			path = "<root>"
		}
		out = append(out, fmt.Sprintf("%s: %s", path, issue.Message))
	}
	return out
}

// IssueMessages returns only the messages of the issues.
func IssueMessages(issues []Issue) []string {
	out := make([]string, 0, len(issues))
	for _, issue := range issues {
		out = append(out, issue.Message)
	}
	return out
}

// ---------------------------------------------------------------- validator

// checker accumulates schema issues. It plays the role of the runtime schemas:
// types are erased at the JSON boundary, the 107 semantics are not.
type checker struct {
	issues []Issue
}

func joinPath(parts ...string) string {
	kept := parts[:0:0]
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, ".")
}

func idx(path string, index int) string {
	return joinPath(path, strconv.Itoa(index))
}

func (c *checker) add(path, message string) {
	c.issues = append(c.issues, Issue{Path: path, Message: message})
}

func (c *checker) merge(prefix string, issues []Issue) {
	for _, issue := range issues {
		c.issues = append(c.issues, Issue{Path: joinPath(prefix, issue.Path), Message: issue.Message})
	}
}

// JSLength is the length of a string in UTF-16 code units, matching how the
// original contracts count characters.
func JSLength(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// JSSlice slices a string by UTF-16 code units, like String.prototype.slice.
func JSSlice(s string, start, end int) string {
	units := utf16.Encode([]rune(s))
	if end > len(units) {
		end = len(units)
	}
	if start > end {
		return ""
	}
	return string(utf16.Decode(units[start:end]))
}

// str checks string length bounds. max < 0 means unbounded.
func (c *checker) str(path, value string, min, max int) bool {
	length := JSLength(value)
	if length < min {
		c.add(path, fmt.Sprintf("Too small: expected string to have >=%d characters", min))
		return false
	}
	if max >= 0 && length > max {
		c.add(path, fmt.Sprintf("Too big: expected string to have <=%d characters", max))
		return false
	}
	return true
}

func (c *checker) optStr(path, value string, min, max int) {
	if value != "" {
		c.str(path, value, min, max)
	}
}

func (c *checker) match(path, value string, re *regexp.Regexp, message string) bool {
	if !re.MatchString(value) {
		c.add(path, message)
		return false
	}
	return true
}

func (c *checker) enum(path, value string, allowed []string) bool {
	for _, candidate := range allowed {
		if candidate == value {
			return true
		}
	}
	quoted := make([]string, len(allowed))
	for i, candidate := range allowed {
		quoted[i] = strconv.Quote(candidate)
	}
	c.add(path, "Invalid option: expected one of "+strings.Join(quoted, "|"))
	return false
}

// arr checks array length bounds. max < 0 means unbounded.
func (c *checker) arr(path string, length, min, max int) bool {
	if length < min {
		c.add(path, fmt.Sprintf("Too small: expected array to have >=%d items", min))
		return false
	}
	if max >= 0 && length > max {
		c.add(path, fmt.Sprintf("Too big: expected array to have <=%d items", max))
		return false
	}
	return true
}

func (c *checker) strings(path string, values []string, minLen, maxLen, minItems, maxItems int) {
	if values == nil && minItems >= 0 {
		c.add(path, "Invalid input: expected array, received undefined")
		return
	}
	c.arr(path, len(values), minItems, maxItems)
	for i, value := range values {
		c.str(idx(path, i), value, minLen, maxLen)
	}
}

func (c *checker) intRange(path string, value, min int, max int, hasMax bool) {
	if value < min {
		c.add(path, fmt.Sprintf("Too small: expected number to be >=%d", min))
	}
	if hasMax && value > max {
		c.add(path, fmt.Sprintf("Too big: expected number to be <=%d", max))
	}
}

func (c *checker) result() []Issue {
	return c.issues
}

// Contains reports whether value is in list.
func Contains(list []string, value string) bool {
	for _, candidate := range list {
		if candidate == value {
			return true
		}
	}
	return false
}

// UniqueStrings returns the distinct values in first-seen order.
func UniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

// CopyStrings returns a non-nil copy so that serialised arrays are [] not null.
func CopyStrings(values []string) []string {
	out := make([]string, len(values))
	copy(out, values)
	return out
}
