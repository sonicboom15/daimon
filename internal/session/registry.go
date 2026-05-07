// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"fmt"
	"sync"
)

var (
	mu        sync.RWMutex
	factories = map[string]SessionFactory{}
)

// Register associates a session store type name with its factory.
// It is intended to be called from component init() functions.
func Register(storeType string, f SessionFactory) {
	mu.Lock()
	defer mu.Unlock()
	factories[storeType] = f
}

// New instantiates a SessionStore of the given type.
func New(storeType string, cfg SessionConfig) (SessionStore, error) {
	mu.RLock()
	f, ok := factories[storeType]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown session store type %q: did you import the component package?", storeType)
	}
	return f(cfg)
}
