// Package user_profilerepo is the data port for the user_profile domain.
//
// The interface lives here; concrete adapters (postgres, in-memory, redis)
// register themselves with samber/do in internal/base/data.
//
// ncgo:domain=user_profile kind=repository
package user_profilerepo

// Repository is the user_profile domain port.
//
// ncgo:domain=user_profile kind=repository
type Repository interface {
	// ncgo:methods:start
	// ncgo:methods:end
}

// Stub is an in-memory Repository implementation suitable for tests and
// for booting a service before a real adapter is wired up. Replace with a
// concrete adapter once the domain ships.
//
// ncgo:domain=user_profile kind=stub
type Stub struct{}

// NewStub returns a Stub Repository.
func NewStub() *Stub { return &Stub{} }

// Compile-time assertion that Stub implements Repository.
var _ Repository = (*Stub)(nil)

// UserProfile is exposed so callers can refer to the domain export name without
// hard-coding the string. It is intentionally a no-op symbol.
const UserProfile = "UserProfile"
