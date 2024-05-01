package middleware

import (
	"context"
	"errors"
	"fmt"

	"github.com/dedpnd/GophKeeper/internal/server/adapters/middleware"
	"github.com/dedpnd/GophKeeper/internal/server/core/domain/proto"
	"github.com/golang-jwt/jwt/v5"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func GetAuthenticator(jwtKey string) func(ctx context.Context) (context.Context, error) {
	return func(ctx context.Context) (context.Context, error) {
		token, err := auth.AuthFromMD(ctx, "bearer")
		if err != nil {
			return nil, fmt.Errorf("AuthFromMD has error: %w", err)
		}

		pl, err := verifyJWTandGetPayload(jwtKey, token)
		if err != nil {
			//nolint:wrapcheck // This legal return
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}

		enCtx := middleware.SetTokenToContext(ctx, pl)

		return enCtx, nil
	}
}

func AuthMatcher(ctx context.Context, callMeta interceptors.CallMeta) bool {
	return proto.User_ServiceDesc.ServiceName != callMeta.Service
}

func verifyJWTandGetPayload(jwtKey string, token string) (middleware.JWTclaims, error) {
	claims := &middleware.JWTclaims{}

	tkn, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtKey), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrSignatureInvalid) {
			return *claims, fmt.Errorf("failed signature from jwt: %w", err)
		}
		return *claims, fmt.Errorf("invalid jwt token: %w", err)
	}

	if !tkn.Valid {
		return *claims, fmt.Errorf("jwt token not valid: %w", err)
	}

	return *claims, nil
}
