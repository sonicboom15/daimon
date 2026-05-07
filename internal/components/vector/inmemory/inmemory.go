// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package inmemory provides an in-process MemoryStore with BM25 lexical scoring.
// It has no external dependencies and is suitable for development and testing.
// Register type: "inmemory".
package inmemory

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/google/uuid"

	"github.com/sonicboom15/daimon/internal/memory"
)

func init() {
	memory.Register("inmemory", func(_ memory.StoreConfig) (memory.MemoryStore, error) {
		return New(), nil
	})
}

const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

type doc struct {
	id       string
	content  string
	metadata map[string]string
	terms    map[string]int
}

// Store is a thread-safe in-memory BM25 vector store.
type Store struct {
	mu   sync.RWMutex
	docs []*doc
}

// New creates an empty in-memory store.
func New() *Store { return &Store{} }

func tokenize(text string) map[string]int {
	counts := make(map[string]int)
	lower := strings.ToLower(text)
	var sb strings.Builder
	for _, r := range lower {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
		} else if sb.Len() > 0 {
			counts[sb.String()]++
			sb.Reset()
		}
	}
	if sb.Len() > 0 {
		counts[sb.String()]++
	}
	return counts
}

func (s *Store) Upsert(_ context.Context, id, content string, metadata map[string]string) (string, error) {
	if id == "" {
		id = uuid.NewString()
	}
	d := &doc{
		id:       id,
		content:  content,
		metadata: metadata,
		terms:    tokenize(content),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.docs {
		if existing.id == id {
			s.docs[i] = d
			return id, nil
		}
	}
	s.docs = append(s.docs, d)
	return id, nil
}

func (s *Store) Query(_ context.Context, query string, topK int) ([]memory.Result, error) {
	qTerms := tokenize(query)

	s.mu.RLock()
	docs := s.docs
	s.mu.RUnlock()

	N := float64(len(docs))
	if N == 0 {
		return nil, nil
	}

	// Average document length (in terms).
	var sumLen float64
	for _, d := range docs {
		var n int
		for _, c := range d.terms {
			n += c
		}
		sumLen += float64(n)
	}
	avgdl := sumLen / N

	// Count documents that contain each query term.
	df := make(map[string]int, len(qTerms))
	for term := range qTerms {
		for _, d := range docs {
			if d.terms[term] > 0 {
				df[term]++
			}
		}
	}

	type scored struct {
		d     *doc
		score float64
	}
	candidates := make([]scored, 0, len(docs))

	for _, d := range docs {
		var docLen int
		for _, c := range d.terms {
			docLen += c
		}
		dl := float64(docLen)
		var score float64
		for term := range qTerms {
			tf := float64(d.terms[term])
			if tf == 0 {
				continue
			}
			n := float64(df[term])
			idf := math.Log((N-n+0.5)/(n+0.5) + 1)
			numerator := tf * (bm25K1 + 1)
			denominator := tf + bm25K1*(1-bm25B+bm25B*dl/avgdl)
			score += idf * numerator / denominator
		}
		if score > 0 {
			candidates = append(candidates, scored{d: d, score: score})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if topK > 0 && len(candidates) > topK {
		candidates = candidates[:topK]
	}

	results := make([]memory.Result, len(candidates))
	for i, c := range candidates {
		results[i] = memory.Result{
			ID:       c.d.id,
			Content:  c.d.content,
			Metadata: c.d.metadata,
			Score:    c.score,
		}
	}
	return results, nil
}

func (s *Store) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, d := range s.docs {
		if d.id == id {
			s.docs = append(s.docs[:i], s.docs[i+1:]...)
			return nil
		}
	}
	return nil
}
