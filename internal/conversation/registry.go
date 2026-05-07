// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package conversation

import (
	"fmt"
	"sync"
)

// ModelConfig holds per-model overrides that a component can apply when the
// request names a specific model.
type ModelConfig struct {
	APIKey string
}

// ComponentDefaults holds inference parameter defaults set in config.yaml.
// Any field left at its zero value means "no default — use the provider's own default."
// Request-level values always override these.
type ComponentDefaults struct {
	Temperature      *float64
	MaxTokens        int
	TopP             *float64
	TopK             *int64
	Stop             []string
	FrequencyPenalty *float64
	PresencePenalty  *float64
	Seed             *int64
	System           string // prepended as a system message when no system message is in the request
}

// ComponentConfig is the full configuration handed to a Factory.
type ComponentConfig struct {
	Metadata    map[string]string
	Models      map[string]ModelConfig
	Defaults    ComponentDefaults
	MemoryStore string // name of the vector store component for RAG enrichment (resolved by server)
}

// Factory creates a Conversation from a ComponentConfig.
type Factory func(cfg ComponentConfig) (Conversation, error)

var (
	mu        sync.RWMutex
	factories = map[string]Factory{}
)

// Register associates a component type name with its factory.
// It is intended to be called from component init() functions.
func Register(componentType string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	factories[componentType] = f
}

// New instantiates a Conversation of the given component type.
func New(componentType string, cfg ComponentConfig) (Conversation, error) {
	mu.RLock()
	f, ok := factories[componentType]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown component type %q: did you import the component package?", componentType)
	}
	return f(cfg)
}
