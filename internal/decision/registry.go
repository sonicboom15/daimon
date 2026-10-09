// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package decision

import (
	"fmt"
	"sync"
)

var (
	mu        sync.RWMutex
	factories = map[string]Factory{}
)

// Register associates a decision type name with its factory.
// It is intended to be called from component init() functions.
func Register(decisionType string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	factories[decisionType] = f
}

// New instantiates a Model of the given type.
func New(decisionType string, cfg Config) (Model, error) {
	mu.RLock()
	f, ok := factories[decisionType]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown decision type %q: did you import the component package?", decisionType)
	}
	return f(cfg)
}

// HasDecision returns true if a factory for decisionType has been registered.
func HasDecision(decisionType string) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := factories[decisionType]
	return ok
}

