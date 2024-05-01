package repository

import (
	"github.com/dedpnd/GophKeeper/internal/server/core/domain"
)

func (s *DB) ReadAllRecord(owner int) ([]*domain.Storage, error) {
	docs := []*domain.Storage{}

	req := s.db.Select("id", "name", "owner").Find(&docs, "owner = ?", owner)
	if req.RowsAffected == 0 {
		return nil, nil
	}

	if req.Error != nil {
		return nil, req.Error
	}

	return docs, nil
}

func (s *DB) ReadRecord(id int, owner int) (*domain.Storage, error) {
	doc := domain.Storage{}

	req := s.db.First(&doc, "id = ? AND owner = ?", id, owner)
	if req.RowsAffected == 0 {
		//nolint:nilnil // This legal return
		return nil, nil
	}

	if req.Error != nil {
		return nil, req.Error
	}

	return &doc, nil
}

func (s *DB) WriteRecord(doc domain.Storage) error {
	req := s.db.Create(&doc)
	if req.Error != nil {
		return req.Error
	}

	return nil
}

func (s *DB) DeleteRecord(id int, owner int) error {
	doc := domain.Storage{}

	req := s.db.Delete(&doc, "id = ? AND owner = ?", id, owner)
	if req.Error != nil {
		return req.Error
	}

	return nil
}
