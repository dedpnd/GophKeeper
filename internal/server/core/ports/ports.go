package ports

import "github.com/dedpnd/GophKeeper/internal/server/core/domain"

type UserRepository interface {
	FindUserByLogin(login string) (*domain.User, error)
	CreateUser(login, hash string) (*domain.User, error)
}

type StorageRepository interface {
	ReadRecord(id int) (*domain.Storage, error)
}
