package repository

import (
	"context"
	"fmt"

	"github.com/dedpnd/GophKeeper/internal/server/core/domain"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DB struct {
	db *gorm.DB
}

func NewDB(ctx context.Context, lg *zap.Logger, dsn string) (*DB, error) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: dsn,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return &DB{}, fmt.Errorf("failed init db session: %w", err)
	}

	// Migrate the schema
	err = db.AutoMigrate(&domain.User{}, &domain.Storage{})
	if err != nil {
		return &DB{}, fmt.Errorf("failed migrate models: %w", err)
	}

	lg.Info(("Connection to postgre: success"))

	return &DB{
		db: db,
	}, nil
}
