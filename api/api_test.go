package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func post(t *testing.T, server *httptest.Server, path string, body any) ActionResponse {
	t.Helper()
	raw, _ := json.Marshal(body)
	res, err := http.Post(server.URL+path, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("%s: status %d", path, res.StatusCode)
	}
	var out ActionResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func captureBody(itemRef, optionID string) map[string]any {
	return map[string]any{
		"instrumentId": "instrument.self-description",
		"itemRef":      itemRef,
		"response":     map[string]any{"kind": "option", "optionId": optionID},
		"declaredContext": []map[string]any{
			{"key": "setting", "value": "employed full-time"},
			{"key": "careerStage", "value": "mid-career"},
		},
		"source":     map[string]any{"kind": "instrument-administration", "ref": "app://session/" + itemRef, "label": "app sitting"},
		"sessionRef": "session.app-1",
	}
}

func TestFullFlowAndRestore(t *testing.T) {
	dir := t.TempDir()
	session, err := NewSession(dir)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewHandler(session))
	defer server.Close()

	out := post(t, server, "/api/capture", captureBody("itm-selfdesc-analysis-1", "opt.fully"))
	if len(out.Rejections) > 0 || len(out.View.Signals) != 1 {
		t.Fatalf("capture failed: %v", out.Rejections)
	}
	signalID := out.View.Signals[0].ID
	out = post(t, server, "/api/decompose", map[string]string{"signalId": signalID})
	if len(out.View.Representations) != 1 || len(out.View.Rendered) != 1 {
		t.Fatalf("decompose failed: %v", out.Rejections)
	}
	representationID := out.View.Representations[0].ID
	out = post(t, server, "/api/map", map[string]any{})
	if len(out.View.PathwaySets) != 1 {
		t.Fatalf("map failed: %v", out.Rejections)
	}
	out = post(t, server, "/api/review", map[string]string{"pathwayId": "path.data-analysis", "state": "interesting"})
	if len(out.View.Decisions) != 1 {
		t.Fatalf("review failed: %v", out.Rejections)
	}
	out = post(t, server, "/api/propose", map[string]string{"representationId": representationID})
	if len(out.View.Proposals) != 1 {
		t.Fatalf("propose failed: %v", out.Rejections)
	}
	out = post(t, server, "/api/correct", map[string]string{"representationId": representationID, "correctedSummary": "studying part-time"})
	if len(out.View.Corrections) != 1 {
		t.Fatalf("correct failed: %v", out.Rejections)
	}

	// The app's one-step answer captures and decomposes together.
	answered := post(t, server, "/api/answer", captureBody("itm-selfdesc-social-1", "opt.mostly"))
	if len(answered.Rejections) > 0 || len(answered.View.Signals) != 2 || len(answered.View.Representations) != 2 {
		t.Fatalf("answer failed: %v", answered.Rejections)
	}

	// Refusals come back as rejections, not as errors.
	refused := post(t, server, "/api/capture", captureBody("itm-selfdesc-analysis-1", "opt.not-an-option"))
	if len(refused.Rejections) == 0 {
		t.Fatal("invalid answer was accepted")
	}

	before := session.View()

	// A new session over the same directory replays the journal.
	restored, err := NewSession(dir)
	if err != nil {
		t.Fatal(err)
	}
	after := restored.View()
	if len(after.Signals) != 2 || after.Signals[0].ID != signalID ||
		after.Representations[0].ID != representationID ||
		after.PathwaySets[0].ID != before.PathwaySets[0].ID ||
		after.Decisions[0].ID != before.Decisions[0].ID ||
		after.Corrections[0].ID != before.Corrections[0].ID ||
		after.Proposals[0].ID != before.Proposals[0].ID ||
		after.Projection.TransformationCount != before.Projection.TransformationCount {
		t.Fatal("restored session differs from the saved one")
	}

	post(t, server, "/api/reset", map[string]any{})
	cleared, _ := NewSession(dir)
	if len(cleared.View().Signals) != 0 {
		t.Fatal("reset did not clear the saved session")
	}
}

func TestCatalogAndResearch(t *testing.T) {
	session, _ := NewSession("")
	server := httptest.NewServer(NewHandler(session))
	defer server.Close()

	res, err := http.Get(server.URL + "/api/catalog")
	if err != nil {
		t.Fatal(err)
	}
	var c Catalog
	_ = json.NewDecoder(res.Body).Decode(&c)
	res.Body.Close()
	if len(c.Items) != 19 || len(c.CapabilityContracts) != 4 || len(c.UncertaintyCodes) != 16 {
		t.Fatalf("catalog incomplete: %d items, %d contracts, %d codes", len(c.Items), len(c.CapabilityContracts), len(c.UncertaintyCodes))
	}

	res, err = http.Post(server.URL+"/api/research/run", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	var r ResearchResponse
	_ = json.NewDecoder(res.Body).Decode(&r)
	res.Body.Close()
	if len(r.Results) != 4 || !r.Reproducibility.Reproducible || r.Report == "" {
		t.Fatal("research run incomplete")
	}

	res, _ = http.Get(server.URL + "/")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("UI not served: %d", res.StatusCode)
	}
	res.Body.Close()
}

func TestChangedAnswerSupersedesAndBudgetDoesNotRunOut(t *testing.T) {
	session, _ := NewSession("")
	server := httptest.NewServer(NewHandler(session))
	defer server.Close()

	post(t, server, "/api/answer", captureBody("itm-selfdesc-analysis-1", "opt.fully"))
	post(t, server, "/api/answer", captureBody("itm-selfdesc-analysis-1", "opt.not-like-me"))
	var out ActionResponse
	for i := 0; i < 66; i++ { // the old budget allowed 64
		out = post(t, server, "/api/map", map[string]any{})
		if len(out.Rejections) > 0 {
			t.Fatalf("map #%d refused: %v", i, out.Rejections)
		}
	}
	set := out.View.PathwaySets[len(out.View.PathwaySets)-1]
	if set.Coverage.RepresentationCount != 1 {
		t.Fatalf("mapped %d representations, want only the latest answer", set.Coverage.RepresentationCount)
	}
	if status := set.Pathways[0].Requirements[0].Status; status != "gap" {
		t.Fatalf("changed answer read as %q, want gap (the latest answer only)", status)
	}
	if len(out.View.Signals) != 2 {
		t.Fatal("the earlier answer must stay in the history")
	}
}
