// Package user_profile implements the user_profile domain usecase layer.
//
// ncgo:domain=user_profile kind=usecase
package user_profile

import (
	user_profilerepo "github.com/x/demo/internal/repository/user_profile"
)

// UseCase is the entry point for the user_profile domain.
//
// ncgo:domain=user_profile kind=usecase
type UseCase struct {
	repo user_profilerepo.Repository
}

// New constructs a UseCase wired to the given Repository.
func New(repo user_profilerepo.Repository) *UseCase {
	return &UseCase{repo: repo}
}

// Repo returns the underlying Repository. Callers should prefer adding
// methods on UseCase rather than reaching through this accessor.
func (u *UseCase) Repo() user_profilerepo.Repository {
	return u.repo
}

// ncgo:methods:start
// ncgo:methods:end
