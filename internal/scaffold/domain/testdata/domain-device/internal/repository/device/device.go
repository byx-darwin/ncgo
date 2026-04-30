// Package devicerepo is the data port for the device domain.
//
// The interface lives here; concrete adapters (postgres, in-memory, redis)
// register themselves with samber/do in internal/base/data.
//
// ncgo:domain=device kind=repository
package devicerepo

// Repository is the device domain port.
//
// ncgo:domain=device kind=repository
type Repository interface {
	// ncgo:methods:start
	// ncgo:methods:end
}

// Stub is an in-memory Repository implementation suitable for tests and
// for booting a service before a real adapter is wired up. Replace with a
// concrete adapter once the domain ships.
//
// ncgo:domain=device kind=stub
type Stub struct{}

// NewStub returns a Stub Repository.
func NewStub() *Stub { return &Stub{} }

// Compile-time assertion that Stub implements Repository.
var _ Repository = (*Stub)(nil)

// Device is exposed so callers can refer to the domain export name without
// hard-coding the string. It is intentionally a no-op symbol.
const Device = "Device"
