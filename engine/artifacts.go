package engine

import (
	"fmt"
	"strconv"
	"sync"

	"career107/domain"
)

func itoa(n int) string { return strconv.Itoa(n) }

// Digest-verified artifact storage.
//
// Artifacts hold S_4 outputs and S_3 payloads. They are addressed by digest
// and verification is explicit: an unverified artifact says so rather than
// implying integrity it has not checked.

type PutArtifactParams struct {
	Value     any
	MediaType string
	CreatedAt string
	CreatedBy string
	SubjectID string
	Hasher    domain.Hasher
}

type ArtifactStore struct {
	mu        sync.RWMutex
	order     []string
	artifacts map[string]Artifact
	contents  map[string]any
	hasher    domain.Hasher
}

func NewArtifactStore(hasher domain.Hasher) *ArtifactStore {
	if hasher == nil {
		hasher = domain.FNV1a64Hasher
	}
	return &ArtifactStore{artifacts: map[string]Artifact{}, contents: map[string]any{}, hasher: hasher}
}

func (s *ArtifactStore) Put(params PutArtifactParams) Artifact {
	hasher := params.Hasher
	if hasher == nil {
		hasher = s.hasher
	}
	canonical := domain.Canonicalize(params.Value)
	digest := domain.DigestOf(hasher, params.Value, "artifact:"+params.MediaType)
	var subject any
	if params.SubjectID != "" {
		subject = params.SubjectID
	}
	id := domain.MintID("art", map[string]any{
		"mediaType": params.MediaType,
		"digest":    digest.Value,
		"subjectId": subject,
	})
	artifact := Artifact{
		ID:         id,
		Digest:     digest,
		MediaType:  params.MediaType,
		ByteLength: domain.UTF8ByteLength(canonical),
		CreatedAt:  params.CreatedAt,
		CreatedBy:  params.CreatedBy,
		SubjectID:  params.SubjectID,
		StorageRef: "memory://artifacts/" + id,
		Integrity:  ArtifactIntegrity{Verified: true, VerifiedAt: params.CreatedAt, Notes: "digest computed at write time"},
	}
	if issues := ValidateArtifact(artifact); len(issues) > 0 {
		panic(fmt.Sprintf("artifact failed validation: %v", domain.IssueStrings(issues)))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.artifacts[id]; !exists {
		s.order = append(s.order, id)
	}
	s.artifacts[id] = artifact
	// The stored content is the canonical form, so later verification reads
	// exactly what was digested and cannot be changed through a shared value.
	s.contents[id] = domain.ToGeneric(params.Value)
	return artifact
}

func (s *ArtifactStore) Get(id string) (Artifact, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	artifact, ok := s.artifacts[id]
	return artifact, ok
}

func (s *ArtifactStore) Content(id string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	content, ok := s.contents[id]
	return content, ok
}

func (s *ArtifactStore) List() []Artifact {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Artifact, 0, len(s.order))
	for _, id := range s.order {
		out = append(out, s.artifacts[id])
	}
	return out
}

// TamperForTesting replaces stored content without updating the digest, so
// tests can prove that verification detects it.
func (s *ArtifactStore) TamperForTesting(id string, content any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contents[id] = domain.ToGeneric(content)
}

type ArtifactVerification struct {
	Verified   bool           `json:"verified"`
	Recorded   *domain.Digest `json:"recorded,omitempty"`
	Recomputed *domain.Digest `json:"recomputed,omitempty"`
}

// Verify re-derives the digest and compares it with the recorded one.
func (s *ArtifactStore) Verify(id string) ArtifactVerification {
	s.mu.RLock()
	artifact, okA := s.artifacts[id]
	content, okC := s.contents[id]
	s.mu.RUnlock()
	if !okA || !okC {
		return ArtifactVerification{Verified: false}
	}
	recomputed := domain.DigestOf(s.hasher, content, "artifact:"+artifact.MediaType)
	recorded := artifact.Digest
	return ArtifactVerification{
		Verified:   recomputed.Value == recorded.Value && recomputed.Algorithm == recorded.Algorithm,
		Recorded:   &recorded,
		Recomputed: &recomputed,
	}
}
