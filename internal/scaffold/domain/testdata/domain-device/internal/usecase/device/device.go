// Package device implements the device domain usecase layer.
//
// ncgo:domain=device kind=usecase
package device

import (
	devicerepo "github.com/x/demo/internal/repository/device"
)

// UseCase is the entry point for the device domain.
//
// ncgo:domain=device kind=usecase
type UseCase struct {
	repo devicerepo.Repository
}

// New constructs a UseCase wired to the given Repository.
func New(repo devicerepo.Repository) *UseCase {
	return &UseCase{repo: repo}
}

// Repo returns the underlying Repository. Callers should prefer adding
// methods on UseCase rather than reaching through this accessor.
func (u *UseCase) Repo() devicerepo.Repository {
	return u.repo
}

// ncgo:methods:start
// ncgo:methods:end
