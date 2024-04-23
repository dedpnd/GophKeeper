package handler

import (
	"context"

	"github.com/dedpnd/GophKeeper/internal/server/core/domain/proto"
	"github.com/dedpnd/GophKeeper/internal/server/core/services"
	"go.uber.org/zap"
)

type StorageHandler struct {
	proto.UnimplementedStorageServer
	Svc    services.StorageService
	Logger *zap.Logger
}

func (s StorageHandler) ReadRecord(ctx context.Context, in *proto.ReadRecordRequest) (*proto.ReadRecordResponse, error) {
	var resp proto.ReadRecordResponse

	rec, err := s.Svc.ReadRecord(int(in.Id))
	if err != nil {
		s.Logger.Error(err.Error())
	}

	resp.Id = int32(rec.ID)

	return &resp, nil
}
