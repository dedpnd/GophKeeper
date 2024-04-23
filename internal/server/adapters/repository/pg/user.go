package repository

import (
	"github.com/dedpnd/GophKeeper/internal/server/core/domain"
)

func (u *DB) FindUserByLogin(login string) (*domain.User, error) {
	user := domain.User{}

	req := u.db.First(&user, "login = ?", login)
	if req.RowsAffected == 0 {
		return nil, nil
	}

	if req.Error != nil {
		return nil, req.Error
	}

	return &user, nil
}

func (u *DB) CreateUser(login, hash string) (*domain.User, error) {
	user := domain.User{
		Login: login,
		Hash:  hash,
	}

	req := u.db.Create(&user)
	if req.Error != nil {
		return nil, req.Error
	}

	return &user, nil
}
