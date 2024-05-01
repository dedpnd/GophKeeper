package repository

import (
	"github.com/dedpnd/GophKeeper/internal/server/core/domain"
)

func (s *DB) FindUserByLogin(login string) (*domain.User, error) {
	user := domain.User{}

	req := s.db.First(&user, "login = ?", login)
	if req.RowsAffected == 0 {
		//nolint:nilnil // This legal return
		return nil, nil
	}

	if req.Error != nil {
		return nil, req.Error
	}

	return &user, nil
}

func (s *DB) CreateUser(login, hash string) (*domain.User, error) {
	user := domain.User{
		Login: login,
		Hash:  hash,
	}

	req := s.db.Create(&user)
	if req.Error != nil {
		return nil, req.Error
	}

	return &user, nil
}
