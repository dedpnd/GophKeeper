package services

import (
	"github.com/dedpnd/GophKeeper/internal/server/core/domain"
	"github.com/dedpnd/GophKeeper/internal/server/core/ports"
)

type StorageService struct {
	repo ports.StorageRepository
}

func NewStorageService(repo ports.StorageRepository) *StorageService {
	return &StorageService{
		repo: repo,
	}
}

func (u *StorageService) ReadRecord(id int) (*domain.Storage, error) {
	return u.repo.ReadRecord(id)
}
