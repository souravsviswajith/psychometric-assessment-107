package domain

import "testing"

func TestCanonicalizeMatchesJSON(t *testing.T) {
	got := Canonicalize(map[string]any{"b": 1.0, "a": []any{"x<y&z", true, nil}, "c": 0.75})
	want := `{"a":["x<y&z",true,null],"b":1,"c":0.75}`
	if got != want {
		t.Fatalf("canonicalize = %s, want %s", got, want)
	}
	if FNV1a64("") != "cbf29ce484222325" {
		t.Fatal("fnv1a64 offset basis wrong")
	}
}

func TestJSNumberAndFixed(t *testing.T) {
	cases := map[float64]string{0: "0", 1: "1", 0.75: "0.75", 1e21: "1e+21", 1e-7: "1e-7", 33.3: "33.3"}
	for in, want := range cases {
		if got := JSNumber(in); got != want {
			t.Errorf("JSNumber(%v) = %s, want %s", in, got, want)
		}
	}
	if JSFixed(0.125, 2) != "0.13" || JSFixed(0.66, 2) != "0.66" || JSFixed(2.5, 0) != "3" {
		t.Fatal("JSFixed does not round ties away from zero like toFixed")
	}
}

func TestFieldGuard(t *testing.T) {
	if ForbiddenFieldReason("religiousAffiliation") == "" || ForbiddenFieldReason("careerStage") != "" {
		t.Fatal("field guard misclassified keys")
	}
}

func TestInterpretOrdinal(t *testing.T) {
	item, _ := FindItem("itm-selfdesc-analysis-1")
	r, problems := InterpretResponse(item, RawResponse{Kind: "option", OptionID: "opt.mostly"})
	if len(problems) > 0 || r.PositionValue() != 0.75 || r.Kind != "ordinal" {
		t.Fatalf("interpretation wrong: %+v %v", r, problems)
	}
	if _, problems := InterpretResponse(item, RawResponse{Kind: "text", Text: "x"}); len(problems) == 0 {
		t.Fatal("wrong response kind accepted")
	}
}
