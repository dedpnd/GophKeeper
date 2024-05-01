//nolint:wrapcheck // This legal return
package services

import (
	"github.com/dedpnd/GophKeeper/internal/server/core/domain"
	"github.com/dedpnd/GophKeeper/internal/server/core/ports"
)

type UserService struct {
	repo ports.UserRepository
}

func NewUserService(repo ports.UserRepository) *UserService {
	return &UserService{
		repo: repo,
	}
}

func (u *UserService) FindUserByLogin(login string) (*domain.User, error) {
	return u.repo.FindUserByLogin(login)
}

func (u *UserService) CreateUser(login, hash string) (*domain.User, error) {
	return u.repo.CreateUser(login, hash)
}
