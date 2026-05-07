// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package embedding

import (
	"fmt"
	"sync"
)

var (
	mu        sync.RWMutex
	factories = map[string]EmbedFactory{}
)

// Register associates an embedder type name with its factory.
// It is intended to be called from component init() functions.
func Register(embedType string, f EmbedFactory) {
	mu.Lock()
	defer mu.Unlock()
	factories[embedType] = f
}

// New instantiates an Embedder of the given type.
func New(embedType string, cfg EmbedConfig) (Embedder, error) {
	mu.RLock()
	f, ok := factories[embedType]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown embedder type %q: did you import the component package?", embedType)
	}
	return f(cfg)
}
