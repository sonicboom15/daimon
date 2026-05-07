// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package memory

import (
	"fmt"
	"sync"
)

var (
	mu        sync.RWMutex
	factories = map[string]StoreFactory{}

	graphMu        sync.RWMutex
	graphFactories = map[string]GraphFactory{}
)

// Register associates a vector store type name with its factory.
// It is intended to be called from component init() functions.
func Register(storeType string, f StoreFactory) {
	mu.Lock()
	defer mu.Unlock()
	factories[storeType] = f
}

// New instantiates a MemoryStore of the given type.
func New(storeType string, cfg StoreConfig) (MemoryStore, error) {
	mu.RLock()
	f, ok := factories[storeType]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown vector store type %q: did you import the component package?", storeType)
	}
	return f(cfg)
}

// RegisterGraph associates a graph store type name with its factory.
// It is intended to be called from component init() functions.
func RegisterGraph(storeType string, f GraphFactory) {
	graphMu.Lock()
	defer graphMu.Unlock()
	graphFactories[storeType] = f
}

// NewGraph instantiates a GraphStore of the given type.
func NewGraph(storeType string, cfg StoreConfig) (GraphStore, error) {
	graphMu.RLock()
	f, ok := graphFactories[storeType]
	graphMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown graph store type %q: did you import the component package?", storeType)
	}
	return f(cfg)
}
