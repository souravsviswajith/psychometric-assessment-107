// Command research runs the 1:3 verification experiments.
//
//	research run     re-run the experiments and rewrite research/results/
//	research check   re-run them and compare against the committed results
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"career107/research"
)

func main() {
	mode := "check"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	dir := "research/results"
	if len(os.Args) > 2 {
		dir = os.Args[2]
	}
	run, err := research.RunAllExperiments()
	if err != nil {
		fail("experiment run failed: %v", err)
	}
	artifact, err := research.MarshalArtifact(research.BuildMachineArtifact(run, time.Now().UTC().Format("2006-01-02T15:04:05.000Z")))
	if err != nil {
		fail("%v", err)
	}
	markdown := research.RenderReportMarkdown(run.Results, &run.Reproducibility)
	jsonPath, mdPath := filepath.Join(dir, "experiments.json"), filepath.Join(dir, "experiments.md")

	switch mode {
	case "run":
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fail("%v", err)
		}
		if err := os.WriteFile(jsonPath, artifact, 0o644); err != nil {
			fail("%v", err)
		}
		if err := os.WriteFile(mdPath, []byte(markdown), 0o644); err != nil {
			fail("%v", err)
		}
		fmt.Printf("Research LOOP executed.\nReport: %s\nData:   %s\n\n", mdPath, jsonPath)
		for _, result := range run.Results {
			fmt.Printf("Experiment %s: %s\n", result.ID, result.Title)
			for _, m := range result.Measures {
				unit := ""
				if m.Unit != "" {
					unit = " " + m.Unit
				}
				fmt.Printf("  - %s: %s%s\n", m.Label, research.FormatValue(m.Value), unit)
			}
		}
		fmt.Printf("\nReproducible fixtures: %v\n", run.Reproducibility.Reproducible)
	case "check":
		committedRaw, err := os.ReadFile(jsonPath)
		if err != nil {
			fail("%v", err)
		}
		committedMd, err := os.ReadFile(mdPath)
		if err != nil {
			fail("%v", err)
		}
		if !run.Reproducibility.Reproducible {
			fail("in-process reproducibility check reported NOT reproducible: %s", strings.Join(run.Reproducibility.Details, "; "))
		}
		if diffs := research.Differences(research.Normalize(decode(committedRaw)), research.Normalize(decode(artifact)), "$", nil); len(diffs) > 0 {
			fail("fresh run differs from %s:\n  %s", jsonPath, strings.Join(diffs, "\n  "))
		}
		if research.NormalizeMarkdown(markdown) != research.NormalizeMarkdown(string(committedMd)) {
			fail("fresh report differs from %s", mdPath)
		}
		measures := 0
		for _, result := range run.Results {
			measures += len(result.Measures)
		}
		fmt.Printf("Reproducibility check passed.\n  experiments compared: %d (%d measures)\n  compared against: %s, %s\n  wall-clock run metadata (generatedAt, \"Ran at\") is excluded; every measurement is compared exactly\n",
			len(run.Results), measures, jsonPath, mdPath)
	default:
		fail("usage: research [run|check] [results-dir]")
	}
}

func decode(raw []byte) any {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var out any
	if err := decoder.Decode(&out); err != nil {
		fail("%v", err)
	}
	return out
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
