// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package conversation

import (
	"fmt"
	"sync"
)

// Factory creates a Conversation from component metadata key/value pairs.
type Factory func(metadata map[string]string) (Conversation, error)

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

// New instantiates a Conversation of the given component type using metadata.
func New(componentType string, metadata map[string]string) (Conversation, error) {
	mu.RLock()
	f, ok := factories[componentType]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown component type %q: did you import the component package?", componentType)
	}
	return f(metadata)
}
