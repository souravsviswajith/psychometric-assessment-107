package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// Privacy field guard.
//
// Career assessment may legitimately use the observation a user volunteers.
// It may not build a surveillance profile out of unrelated personal
// characteristics (§18 / §17). The guard is repository-wide so that no module
// can quietly introduce a prohibited field: career context keys, evidence
// notes, uncertainty details, and generated pathway factors are all checked.
//
// The guard is a structural control, not a legal determination.

// ForbiddenFieldKeys are substrings (on normalised keys) that must not enter
// career assessment representations.
var ForbiddenFieldKeys = []string{
	"race",
	"racial",
	"ethnicity",
	"ethnicorigin",
	"nationalorigin",
	"religion",
	"religious",
	"spiritualbelief",
	"sexualorientation",
	"genderidentity",
	"maritalstatus",
	"marriageintent",
	"pregnancy",
	"pregnant",
	"disabilitystatus",
	"medicalcondition",
	"diagnosis",
	"healthstatus",
	"mentalhealthdiagnosis",
	"medication",
	"geneticinformation",
	"biometric",
	"fingerprint",
	"facialrecognition",
	"politicalaffiliation",
	"partyaffiliation",
	"unionmembership",
	"criminalrecord",
	"creditreport",
	"income",
	"networth",
	"bankbalance",
	"caregiverstatus",
	"immigrationstatus",
	"citizenshipstatus",
}

var nonLetters = regexp.MustCompile(`[^a-z]`)

func normalizeKey(key string) string {
	return nonLetters.ReplaceAllString(strings.ToLower(key), "")
}

// ForbiddenFieldReason returns a human-readable reason when a key is
// prohibited, otherwise "".
func ForbiddenFieldReason(key string) string {
	normalized := normalizeKey(key)
	for _, forbidden := range ForbiddenFieldKeys {
		if strings.Contains(normalized, forbidden) {
			return fmt.Sprintf("field key %q is outside the career-assessment boundary (matched prohibited characteristic %q)", key, forbidden)
		}
	}
	return ""
}

var contextKeyPattern = regexp.MustCompile(`^[a-z][a-zA-Z0-9-]*$`)

func checkContextFieldKey(c *checker, path, key string) {
	if !c.str(path, key, 1, 64) {
		return
	}
	if !c.match(path, key, contextKeyPattern, "field keys must be lowerCamelCase or kebab-case, max 64 chars") {
		return
	}
	if ForbiddenFieldReason(key) != "" {
		c.add(path, "field key is outside the career-assessment boundary")
	}
}

// CheckProseField validates free prose that may be recorded alongside a
// representation.
func CheckProseField(text string) []Issue {
	c := &checker{}
	if c.str("", text, 1, 500) && ForbiddenFieldReason(text) != "" {
		c.add("", "text asks for or records a characteristic outside the career-assessment boundary")
	}
	return c.result()
}
