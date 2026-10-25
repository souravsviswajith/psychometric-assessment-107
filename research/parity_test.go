package research

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestParityWithCommittedResults re-runs the experiments and compares every
// measurement — including the deterministic signal identities — against the
// artifacts the original TypeScript implementation committed. Passing means
// the Go port reproduces the original's ids, digests, clock usage, mapping
// and experiment logic exactly.
func TestParityWithCommittedResults(t *testing.T) {
	run, err := RunAllExperiments()
	if err != nil {
		t.Fatal(err)
	}
	if !run.Reproducibility.Reproducible {
		t.Fatalf("fixture not reproducible: %v", run.Reproducibility.Details)
	}

	committedRaw, err := os.ReadFile("results/experiments.json")
	if err != nil {
		t.Fatal(err)
	}
	var committed any
	decoder := json.NewDecoder(bytes.NewReader(committedRaw))
	decoder.UseNumber()
	if err := decoder.Decode(&committed); err != nil {
		t.Fatal(err)
	}

	freshRaw, err := MarshalArtifact(BuildMachineArtifact(run, "ignored"))
	if err != nil {
		t.Fatal(err)
	}
	var fresh any
	decoder = json.NewDecoder(bytes.NewReader(freshRaw))
	decoder.UseNumber()
	if err := decoder.Decode(&fresh); err != nil {
		t.Fatal(err)
	}

	if diffs := Differences(Normalize(committed), Normalize(fresh), "$", nil); len(diffs) > 0 {
		t.Fatalf("fresh run differs from committed experiments.json:\n  %s", strings.Join(diffs, "\n  "))
	}

	committedMarkdown, err := os.ReadFile("results/experiments.md")
	if err != nil {
		t.Fatal(err)
	}
	freshMarkdown := RenderReportMarkdown(run.Results, &run.Reproducibility)
	if NormalizeMarkdown(freshMarkdown) != NormalizeMarkdown(string(committedMarkdown)) {
		want := strings.Split(NormalizeMarkdown(string(committedMarkdown)), "\n")
		got := strings.Split(NormalizeMarkdown(freshMarkdown), "\n")
		for i := 0; i < len(want) || i < len(got); i++ {
			var w, g string
			if i < len(want) {
				w = want[i]
			}
			if i < len(got) {
				g = got[i]
			}
			if w != g {
				t.Fatalf("experiments.md differs at line %d:\n  committed: %q\n  fresh:     %q", i+1, w, g)
			}
		}
	}
}
