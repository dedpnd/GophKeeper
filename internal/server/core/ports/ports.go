package ports

import "github.com/dedpnd/GophKeeper/internal/server/core/domain"

type UserRepository interface {
	FindUserByLogin(login string) (*domain.User, error)
	CreateUser(login, hash string) (*domain.User, error)
}

type StorageRepository interface {
	ReadRecord(id int, owner int) (*domain.Storage, error)
	ReadAllRecord(owner int) ([]*domain.Storage, error)
	WriteRecord(doc domain.Storage) error
	DeleteRecord(id int, owner int) error
}
