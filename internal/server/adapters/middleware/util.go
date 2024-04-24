package middleware

import (
	"context"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey int

type JWTclaims struct {
	ID    int    `json:"id"`
	Login string `json:"login"`
	jwt.RegisteredClaims
}

const (
	ContextKeyToken contextKey = iota
)

func GetTokenFromContext(ctx context.Context) (JWTclaims, bool) {
	caller, ok := ctx.Value(ContextKeyToken).(JWTclaims)
	return caller, ok
}

func SetTokenToContext(ctx context.Context, pl JWTclaims) context.Context {
	return context.WithValue(ctx, ContextKeyToken, pl)
}
