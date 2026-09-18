package daemon

import (
	"net/http"
	"sync"

	"github.com/docker/secrets-engine/x/secrets"
)

type PluginEntry struct {
	Name    string
	Pattern secrets.Pattern
	Client  *http.Client
}

type Registry struct {
	mu      sync.RWMutex
	plugins map[string]PluginEntry
}

func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]PluginEntry),
	}
}

// Register stores the plugin, replacing any previous entry under the same name so a reconnecting plugin gets its new client.
func (r *Registry) Register(name string, pattern secrets.Pattern, client *http.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins[name] = PluginEntry{
		Name:    name,
		Pattern: pattern,
		Client:  client,
	}
}

func (r *Registry) FindForPattern(pattern secrets.Pattern) (PluginEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.plugins {
		if p.Pattern.Overlaps(pattern) {
			return p, true
		}
	}
	return PluginEntry{}, false
}

func (r *Registry) SetClient(name string, client *http.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.plugins[name]; ok {
		entry.Client = client
		r.plugins[name] = entry
	}
}

func (r *Registry) List() []PluginEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]PluginEntry, 0, len(r.plugins))
	for _, p := range r.plugins {
		result = append(result, p)
	}
	return result
}
