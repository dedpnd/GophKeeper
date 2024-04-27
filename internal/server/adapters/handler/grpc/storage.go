package handler

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/dedpnd/GophKeeper/internal/server/adapters/middleware"
	"github.com/dedpnd/GophKeeper/internal/server/core/domain"
	"github.com/dedpnd/GophKeeper/internal/server/core/domain/proto"
	"github.com/dedpnd/GophKeeper/internal/server/core/services"
	"go.uber.org/zap"
)

type StorageHandler struct {
	proto.UnimplementedStorageServer
	Svc       services.StorageService
	Logger    *zap.Logger
	MasterKey string
}

// ReadAllRecord read all record from BD
func (s StorageHandler) ReadAllRecord(ctx context.Context, in *proto.ReadAllRecordRequest) (*proto.ReadAllRecordResponse, error) {
	var resp proto.ReadAllRecordResponse

	// Get token from context
	token, ok := middleware.GetTokenFromContext(ctx)
	if !ok {
		s.Logger.Error("invalid token")
		resp.Error = "invalid token"
		return &resp, nil
	}

	// Get data from BD
	rec, err := s.Svc.ReadAllRecord(token.ID)
	if err != nil {
		s.Logger.With(zap.Error(err)).Error("failed get all records")
		resp.Error = "failed get all records"
		return &resp, nil
	}

	// Preparing response
	respSlice := make([]*proto.StorageUnit, len(rec))
	for _, v := range rec {
		respSlice = append(respSlice, &proto.StorageUnit{
			Id:    int32(v.ID),
			Name:  v.Name,
			Type:  v.Type,
			Owner: int32(v.Owner),
		})
	}

	resp.Units = respSlice
	return &resp, nil
}

// ReadRecord read single record from BD
func (s StorageHandler) ReadRecord(ctx context.Context, in *proto.ReadRecordRequest) (*proto.ReadRecordResponse, error) {
	var resp proto.ReadRecordResponse

	// Get token from context
	token, ok := middleware.GetTokenFromContext(ctx)
	if !ok {
		s.Logger.Error("invalid token")
		resp.Error = "invalid token"
		return &resp, nil
	}

	// Get record from BD
	rec, err := s.Svc.ReadRecord(int(in.Id), token.ID)
	if err != nil {
		s.Logger.With(zap.Error(err)).Error("failed read record")
		resp.Error = "failed read record"
		return &resp, nil
	}

	if rec == nil {
		resp.Error = "record not found"
		return &resp, nil
	}

	// Dectyption data
	data, err := decryptionData(s.MasterKey, rec.Key, rec.Value)
	if err != nil {
		s.Logger.With(zap.Error(err)).Error("failed decrypt data")
		resp.Error = "failed decrypt data"
		return &resp, nil
	}

	resp.Data = data

	return &resp, nil
}

// WriteRecord write record in BD
func (s StorageHandler) WriteRecord(stream proto.Storage_WriteRecordServer) error {
	var resp proto.WriteRecordResponse
	var fileName string
	var fileType string

	// For chunk
	buffer := &bytes.Buffer{}

	// Get token from context
	token, ok := middleware.GetTokenFromContext(stream.Context())
	if !ok {
		s.Logger.Error("invalid token")
		resp.Error = "invalid token"
		return stream.SendAndClose(&resp)
	}

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.Logger.With(zap.Error(err)).Error("failed recive chunk")
			resp.Error = "failed recive chunk"
			return stream.SendAndClose(&resp)
		}

		// Saving the file name from the request
		if fileName == "" {
			fileName = chunk.GetName()
		}

		if fileType == "" {
			fileType = chunk.GetType()
		}

		// Write the data to the buffer
		if _, err := buffer.Write(chunk.GetData()); err != nil {
			s.Logger.With(zap.Error(err)).Error("failed write chunk to buffer")
			resp.Error = "failed write chunk to buffer"
			return stream.SendAndClose(&resp)
		}
	}

	// Encription data
	data, key, err := encryptionData(s.MasterKey, buffer.Bytes())
	if err != nil {
		s.Logger.With(zap.Error(err)).Error("failed encrypt data")
		resp.Error = "failed encrypt data"
		return stream.SendAndClose(&resp)
	}

	// Prepare record for save
	var unit = domain.Storage{
		Name:  fileName,
		Type:  fileType,
		Value: data,
		Key:   key,
		Owner: token.ID,
	}

	// Write recorn in BD
	err = s.Svc.WriteRecord(unit)
	if err != nil {
		s.Logger.With(zap.Error(err)).Error("failed write record")
		resp.Error = "failed write record"
		return stream.SendAndClose(&resp)
	}

	// Close stream
	return stream.SendAndClose(&resp)
}

// DeleteRecord delete record from BD
func (s StorageHandler) DeleteRecord(ctx context.Context, in *proto.DeleteRecordRequest) (*proto.DeleteRecordResponse, error) {
	var resp proto.DeleteRecordResponse

	// Get token from context
	token, ok := middleware.GetTokenFromContext(ctx)
	if !ok {
		s.Logger.Error("invalid token")
		resp.Error = "invalid token"
		return &resp, nil
	}

	// Delete record
	err := s.Svc.DeleteRecord(int(in.Id), token.ID)
	if err != nil {
		s.Logger.With(zap.Error(err)).Error("failed delete record")
		resp.Error = "failed delete record"
		return &resp, nil
	}

	return &resp, nil
}

/* UTILS */

func encryptionData(mk string, data []byte) (string, string, error) {
	key, err := generateRandom(16)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	encKey, err := encrypt([]byte(mk), key)
	if err != nil {
		return "", "", fmt.Errorf("failed encript key: %w", err)
	}

	encData, err := encrypt(key, data)
	if err != nil {
		return "", "", fmt.Errorf("failed encript data: %w", err)
	}

	return encData, encKey, nil
}

func decryptionData(mk string, key string, data string) ([]byte, error) {
	decKey, err := decrypt([]byte(mk), key)
	if err != nil {
		return []byte{}, fmt.Errorf("failed decrypt key: %w", err)
	}

	decData, err := decrypt([]byte(decKey), data)
	if err != nil {
		return []byte{}, fmt.Errorf("failed decrypt data: %w", err)
	}

	return decData, nil
}

func encrypt(key []byte, plaintext []byte) (string, error) {
	// Преобразуйте ключ в байты нужной длины
	keyBytes := adjustKeySize(key, 16)
	// Создайте новый блок AES с использованием ключа
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// NewGCM возвращает заданный 128-битный блочный шифр
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create chiper: %w", err)
	}

	// Создаём вектор инициализации
	nonce, err := generateRandom(aesgcm.NonceSize())
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	dst := aesgcm.Seal(nil, nonce, plaintext, nil)

	// Кодируем зашифрованные данные в строку (base64)
	encString := base64.StdEncoding.EncodeToString(nonce) + "*" + base64.StdEncoding.EncodeToString(dst)

	return encString, nil
}

func decrypt(key []byte, plaintext string) ([]byte, error) {
	splStr := strings.Split(plaintext, "*")

	// Получаем вектор
	decNonce, err := base64.StdEncoding.DecodeString(splStr[0])
	if err != nil {
		return []byte{}, fmt.Errorf("failed decode base64: %w", err)
	}

	// Зашифровваные данные
	decString, err := base64.StdEncoding.DecodeString(splStr[1])
	if err != nil {
		return []byte{}, fmt.Errorf("failed decode base64: %w", err)
	}

	// Преобразуйте ключ в байты нужной длины
	keyBytes := adjustKeySize([]byte(key), 16)
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// NewGCM возвращает заданный 128-битный блочный шифр
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to create chiper: %w", err)
	}

	// Расшифровываем
	dst, err := aesgcm.Open(nil, decNonce, decString, nil)
	if err != nil {
		return []byte{}, fmt.Errorf("failed open decrypts: %w", err)
	}

	return dst, nil
}

func adjustKeySize(originalKey []byte, desiredSize int) []byte {
	// Если исходный ключ больше желаемого размера, обрезаем его
	if len(originalKey) > desiredSize {
		return originalKey[:desiredSize]
	}

	return originalKey
}

func generateRandom(size int) ([]byte, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		return nil, fmt.Errorf("failed generate byte: %w", err)
	}

	return b, nil
}
