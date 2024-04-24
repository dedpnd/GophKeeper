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

func (s *StorageService) ReadAllRecord(owner int) ([]*domain.Storage, error) {
	return s.repo.ReadAllRecord(owner)
}

func (s *StorageService) ReadRecord(id int, owner int) (*domain.Storage, error) {
	return s.repo.ReadRecord(id, owner)
}

func (s *StorageService) WriteRecord(doc domain.Storage) error {
	return s.repo.WriteRecord(doc)
}

func (s *StorageService) DeleteRecord(id int, owner int) error {
	return s.repo.DeleteRecord(id, owner)
}
