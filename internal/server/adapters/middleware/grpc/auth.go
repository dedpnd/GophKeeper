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
			return nil, err
		}

		pl, isAuth := verifyJWTandGetPayload(jwtKey, token)
		if !isAuth {
			return nil, status.Error(codes.Unauthenticated, "invalid auth token")
		}

		enCtx := middleware.SetTokenToContext(ctx, pl)

		return enCtx, nil
	}
}

func AuthMatcher(ctx context.Context, callMeta interceptors.CallMeta) bool {
	return proto.User_ServiceDesc.ServiceName != callMeta.Service
}

func verifyJWTandGetPayload(jwtKey string, token string) (middleware.JWTclaims, bool) {
	claims := &middleware.JWTclaims{}

	tkn, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtKey), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrSignatureInvalid) {
			fmt.Printf("failed signature from jwt: %s", err)
			return *claims, false
		}
		fmt.Printf("invalid jwt token: %s", err)
		return *claims, false
	}

	if !tkn.Valid {
		fmt.Printf("jwt token not valid: %s", err)
		return *claims, false
	}

	return *claims, true
}
