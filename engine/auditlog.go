package engine

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"career107/domain"
)

// Append-only audit log with a cryptographic digest chain.
//
// The domain deliberately ships a non-cryptographic digest and labels it as
// such. Production audit trails need a cryptographic digest, so the log uses
// SHA-256 — with the same Hasher interface and with its strength stated on
// every digest it produces.
//
// The log itself is append-only JSON Lines: a line is never rewritten, and
// each line carries the digest of the previous line, so removal or reordering
// is detectable.

type AuditEntry struct {
	Sequence            int            `json:"sequence"`
	At                  string         `json:"at"`
	Actor               string         `json:"actor"`
	Operation           string         `json:"operation"`
	SubjectID           string         `json:"subjectId,omitempty"`
	PayloadDigest       domain.Digest  `json:"payloadDigest"`
	PreviousEntryDigest *domain.Digest `json:"previousEntryDigest,omitempty"`
	Notes               string         `json:"notes,omitempty"`
}

type AppendAuditParams struct {
	At        string
	Actor     string
	Operation string
	Payload   any
	SubjectID string
	Notes     string
}

type AuditLog struct {
	mu       sync.Mutex
	path     string
	sequence int
	previous *domain.Digest
	entries  []AuditEntry
}

func NewAuditLog(path string) *AuditLog {
	return &AuditLog{path: path}
}

func (l *AuditLog) Append(params AppendAuditParams) (AuditEntry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sequence++
	entry := AuditEntry{
		Sequence:            l.sequence,
		At:                  params.At,
		Actor:               params.Actor,
		Operation:           params.Operation,
		SubjectID:           params.SubjectID,
		PayloadDigest:       domain.DigestOf(domain.SHA256Hasher, params.Payload, fmt.Sprintf("audit:%s:%d", params.Operation, l.sequence)),
		PreviousEntryDigest: l.previous,
		Notes:               params.Notes,
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return AuditEntry{}, err
	}
	file, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return AuditEntry{}, err
	}
	defer file.Close()
	if _, err := file.WriteString(domain.Canonicalize(entry) + "\n"); err != nil {
		return AuditEntry{}, err
	}
	digest := domain.DigestOf(domain.SHA256Hasher, entry, fmt.Sprintf("audit-entry:%d", l.sequence))
	l.previous = &digest
	l.entries = append(l.entries, entry)
	return entry, nil
}

func (l *AuditLog) InMemoryEntries() []AuditEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]AuditEntry{}, l.entries...)
}

type AuditVerification struct {
	Valid          bool     `json:"valid"`
	CheckedEntries int      `json:"checkedEntries"`
	Problems       []string `json:"problems"`
}

// Verify checks the on-disk chain: every entry must digest the one before it.
func (l *AuditLog) Verify() (AuditVerification, error) {
	file, err := os.Open(l.path)
	if err != nil {
		return AuditVerification{}, err
	}
	defer file.Close()
	problems := []string{}
	var previous *domain.Digest
	checked := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		checked++
		var entry AuditEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			problems = append(problems, fmt.Sprintf("line %d is not valid JSON", checked))
			continue
		}
		if previous != nil && (entry.PreviousEntryDigest == nil || entry.PreviousEntryDigest.Value != previous.Value) {
			problems = append(problems, fmt.Sprintf("line %d does not chain to the previous entry", checked))
		}
		digest := domain.DigestOf(domain.SHA256Hasher, entry, fmt.Sprintf("audit-entry:%d", entry.Sequence))
		previous = &digest
	}
	if err := scanner.Err(); err != nil {
		return AuditVerification{}, err
	}
	return AuditVerification{Valid: len(problems) == 0, CheckedEntries: checked, Problems: problems}, nil
}
