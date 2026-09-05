package framework

import "fmt"

// registry is a lookup table from kind to Adapter. The zero value is not
// usable; construct with newRegistry. Mirrors internal/scaffold/infra's
// pluginRegistry pattern (see internal/scaffold/infra/infra.go:90-131).
type registry struct {
	byKind map[string]Adapter
}

func newRegistry() *registry {
	return &registry{byKind: map[string]Adapter{}}
}

func (r *registry) register(a Adapter) {
	if _, exists := r.byKind[a.Kind()]; exists {
		panic(fmt.Sprintf("framework: duplicate adapter registration for kind %q", a.Kind()))
	}
	r.byKind[a.Kind()] = a
}

func (r *registry) get(kind string) (Adapter, bool) {
	a, ok := r.byKind[kind]
	return a, ok
}

var defaultRegistry = newRegistry()

// Register adds an adapter to the package-level registry. Called from each
// adapter file's init(). Panics on duplicate Kind() — the adapter set is
// closed and known at compile time, so a collision is a programming error.
func Register(a Adapter) { defaultRegistry.register(a) }

// Get looks up the adapter for kind (e.g. manifest.KindHertz).
func Get(kind string) (Adapter, bool) { return defaultRegistry.get(kind) }

// MustGet looks up the adapter for kind and panics if none is registered.
// Callers use this only where kind has already been validated (e.g. after
// Options.validate() or manifest.Manifest.Validate() succeeded).
func MustGet(kind string) Adapter {
	a, ok := Get(kind)
	if !ok {
		panic(fmt.Sprintf("framework: no adapter registered for kind %q", kind))
	}
	return a
}
