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

// ComponentConfig is the full configuration handed to a Factory.
// Metadata carries flat key/value pairs from the YAML; Models carries
// per-model overrides keyed by model name.
type ComponentConfig struct {
	Metadata map[string]string
	Models   map[string]ModelConfig
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
