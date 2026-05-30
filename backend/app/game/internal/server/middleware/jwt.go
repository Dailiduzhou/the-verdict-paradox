package middleware

import (
	"context"

	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/biz"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	kratosjwt "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
)

func CheckBlacklist(authUc *biz.AuthUsecase) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			claims, ok := kratosjwt.FromContext(ctx)
			if !ok {
				return nil, errors.Unauthorized("AUTH_ERROR", "missing token")
			}

			gc, ok := claims.(*biz.GameClaims)
			if !ok {
				return nil, errors.Unauthorized("AUTH_ERROR", "invalid token claims")
			}

			if gc.ID == "" {
				return nil, errors.Unauthorized("AUTH_ERROR", "missing token id")
			}

			blacklisted, err := authUc.IsTokenBlacklisted(ctx, gc.ID)
			if err != nil {
				return nil, errors.InternalServer("AUTH_ERROR", "check blacklist failed")
			}
			if blacklisted {
				return nil, errors.Unauthorized("AUTH_ERROR", "token has been revoked")
			}

			return handler(ctx, req)
		}
	}
}

type keyClaims struct{}

func WithClaims(ctx context.Context, claims *biz.GameClaims) context.Context {
	return context.WithValue(ctx, keyClaims{}, claims)
}

func ClaimsFromContext(ctx context.Context) (*biz.GameClaims, bool) {
	claims, ok := ctx.Value(keyClaims{}).(*biz.GameClaims)
	return claims, ok
}

func InjectClaims() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			if claims, ok := kratosjwt.FromContext(ctx); ok {
				if gc, ok := claims.(*biz.GameClaims); ok {
					ctx = WithClaims(ctx, gc)
				}
			}
			return handler(ctx, req)
		}
	}
}
