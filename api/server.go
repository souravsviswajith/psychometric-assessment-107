package api

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"career107/domain"
	"career107/research"
)

// webdist holds the built web UI. `make web` (or scripts/build-web.sh)
// populates it before the Go binary or the mobile library is built.
//
//go:embed all:webdist
var webdist embed.FS

// Catalog is the static domain vocabulary the UI renders: instrument items,
// contracts, invariants, and the statements the UI must show verbatim.
type Catalog struct {
	Items                     []domain.InstrumentItem       `json:"items"`
	CapabilityContracts       []domain.CapabilityContract   `json:"capabilityContracts"`
	ContractVersions          []domain.ContractVersionEntry `json:"contractVersions"`
	InvariantDescriptors      []domain.InvariantDescriptor  `json:"invariantDescriptors"`
	UncertaintyCodes          []string                      `json:"uncertaintyCodes"`
	ForbiddenScoreKeys        []string                      `json:"forbiddenScoreKeys"`
	VocabularyID              string                        `json:"vocabularyId"`
	UserAuthorityStatement    string                        `json:"userAuthorityStatement"`
	NoScalarCollapseStatement string                        `json:"noScalarCollapseStatement"`
	MappingPolicy             domain.MappingPolicy          `json:"mappingPolicy"`
	SubjectID                 string                        `json:"subjectId"`
	SubjectLabel              string                        `json:"subjectLabel"`
	RuntimeID                 string                        `json:"runtimeId"`
}

func catalog() Catalog {
	return Catalog{
		Items:                     domain.AllItems(),
		CapabilityContracts:       domain.CapabilityContracts,
		ContractVersions:          domain.ContractVersions,
		InvariantDescriptors:      domain.InvariantDescriptors,
		UncertaintyCodes:          domain.UncertaintyCodes(),
		ForbiddenScoreKeys:        domain.ForbiddenScoreKeys,
		VocabularyID:              domain.VocabularyID,
		UserAuthorityStatement:    domain.UserAuthorityStatement,
		NoScalarCollapseStatement: domain.NoScalarCollapseStatement,
		MappingPolicy:             domain.DefaultMappingPolicy,
		SubjectID:                 SubjectID,
		SubjectLabel:              SubjectLabel,
		RuntimeID:                 SessionRuntime,
	}
}

// ActionResponse is returned by every mutation: what happened, what was
// refused, and the session state afterwards.
type ActionResponse struct {
	Notice     string      `json:"notice,omitempty"`
	Rejections []string    `json:"rejections"`
	View       SessionView `json:"view"`
}

type ResearchResponse struct {
	Results         []research.ExperimentResult    `json:"results"`
	Reproducibility research.ReproducibilityReport `json:"reproducibility"`
	Report          string                         `json:"report"`
}

const maxRequestBytes = 64 << 10

// NewHandler serves the API under /api/ and the web UI everywhere else.
func NewHandler(session *Session) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "runtime": SessionRuntime})
	})
	mux.HandleFunc("GET /api/catalog", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, catalog())
	})
	mux.HandleFunc("GET /api/session", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, session.View())
	})

	action := func(op string, decode func(*http.Request) (any, error)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
			args, err := decode(r)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			notice, rejections, err := session.Do(op, args)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			if rejections == nil {
				rejections = []string{}
			}
			writeJSON(w, http.StatusOK, ActionResponse{Notice: notice, Rejections: rejections, View: session.View()})
		}
	}
	mux.HandleFunc("POST /api/capture", action("capture", decodeCapture))
	mux.HandleFunc("POST /api/answer", action("answer", decodeCapture))
	mux.HandleFunc("POST /api/decompose", action("decompose", decodeNew[decomposeArgs]))
	mux.HandleFunc("POST /api/map", action("map", func(*http.Request) (any, error) { return struct{}{}, nil }))
	mux.HandleFunc("POST /api/review", action("review", decodeNew[reviewArgs]))
	mux.HandleFunc("POST /api/correct", action("correct", decodeNew[correctArgs]))
	mux.HandleFunc("POST /api/propose", action("propose", decodeNew[proposeArgs]))

	mux.HandleFunc("POST /api/reset", func(w http.ResponseWriter, r *http.Request) {
		if err := session.Reset(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		notice := "Session reset. Nothing from the previous session remains on this device."
		writeJSON(w, http.StatusOK, ActionResponse{Notice: notice, Rejections: []string{}, View: session.View()})
	})

	mux.HandleFunc("POST /api/research/run", func(w http.ResponseWriter, r *http.Request) {
		run, err := research.RunAllExperiments()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, ResearchResponse{
			Results:         run.Results,
			Reproducibility: run.Reproducibility,
			Report:          research.RenderReportMarkdown(run.Results, &run.Reproducibility),
		})
	})

	mux.Handle("/", staticHandler())
	return logRequests(mux)
}

// staticHandler serves the embedded web build, falling back to index.html so
// client-side routes work.
func staticHandler() http.Handler {
	dist, err := fs.Sub(webdist, "webdist")
	if err != nil {
		panic(err)
	}
	files := http.FileServerFS(dist)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if _, err := fs.Stat(dist, path); err == nil {
				files.ServeHTTP(w, r)
				return
			}
		}
		index, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			http.Error(w, "web UI not built: run `make web` before building the server", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}

// decodeCapture reads an answer. The session owns the subject, and the server
// mints the input identity: neither is taken from the client.
func decodeCapture(r *http.Request) (any, error) {
	var input domain.SignalInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		return nil, err
	}
	input.SubjectID = SubjectID
	if input.InputID == "" {
		input.InputID = domain.MintID("inp", map[string]any{
			"session":  "app",
			"itemRef":  input.ItemRef,
			"at":       domain.FormatIso(time.Now()),
			"response": input.Response,
		})
	}
	return captureArgs{Input: input}, nil
}

// decodeNew decodes the body into a fresh T for every request.
func decodeNew[T any](r *http.Request) (any, error) {
	var target T
	if err := json.NewDecoder(r.Body).Decode(&target); err != nil {
		return nil, err
	}
	return target, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
		}
	})
}
