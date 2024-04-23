package repository

import (
	"context"
	"fmt"

	"github.com/dedpnd/GophKeeper/internal/server/core/domain"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DB struct {
	db *gorm.DB
}

func NewDB(ctx context.Context, lg *zap.Logger, dsn string) (*DB, error) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: dsn,
	}), &gorm.Config{})
	if err != nil {
		return &DB{}, fmt.Errorf("failed init db session: %w", err)
	}

	// Migrate the schema
	db.AutoMigrate(&domain.User{}, &domain.Storage{})

	lg.Info(("Connection to postgre: success"))

	return &DB{
		db: db,
	}, nil
}
