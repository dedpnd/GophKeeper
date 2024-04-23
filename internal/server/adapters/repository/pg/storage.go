package repository

import (
	"github.com/dedpnd/GophKeeper/internal/server/core/domain"
)

// TODO: test
func (s DB) ReadRecord(id int) (*domain.Storage, error) {
	return &domain.Storage{
		ID:      1,
		Message: "123",
	}, nil
}
