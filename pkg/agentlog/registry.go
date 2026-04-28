package agentlog

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

type BackendFactory func() StorageBackend

var (
	registryMu sync.RWMutex
	registry   = map[string]BackendFactory{}
)

func RegisterBackend(name string, factory BackendFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = factory
}

func BackendNames() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for k := range registry {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func CreateBackend(ctx context.Context, name string, cfg map[string]string) (StorageBackend, error) {
	registryMu.RLock()
	factory, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("backend %q not registered", name)
	}
	b := factory()
	if err := b.Init(ctx, cfg); err != nil {
		return nil, err
	}
	return b, nil
}
