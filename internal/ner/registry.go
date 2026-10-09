// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package ner

import (
	"fmt"
	"sync"
)

var (
	mu        sync.RWMutex
	factories = map[string]Factory{}
)

// Register associates an NER type name with its factory.
// It is intended to be called from component init() functions.
func Register(nerType string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	factories[nerType] = f
}

// New instantiates a Model of the given type.
func New(nerType string, cfg Config) (Model, error) {
	mu.RLock()
	f, ok := factories[nerType]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown ner type %q: did you import the component package?", nerType)
	}
	return f(cfg)
}

// HasNER returns true if a factory for nerType has been registered.
func HasNER(nerType string) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := factories[nerType]
	return ok
}

