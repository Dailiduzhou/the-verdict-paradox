package server

import (
	"context"

	userv1 "github.com/Dailiduzhou/the-verdict-paradox/backend/api/user/v1"
	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/biz"
	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/conf"
	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/service"

	custommid "github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/server/middleware"

	"github.com/go-kratos/kratos/v2/log"
	kratosjwt "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport/http"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

var publicOps = map[string]bool{
	userv1.OperationUserRegister:     true,
	userv1.OperationUserLogin:        true,
	userv1.OperationUserVerify:       true,
	userv1.OperationUserRefreshToken: true,
}

// NewHTTPServer new an HTTP server.
func NewHTTPServer(c *conf.Server, user *service.UserService, authUc *biz.AuthUsecase, ac *conf.Auth, logger log.Logger) *http.Server {
	jwtMiddleware := kratosjwt.Server(
		func(t *jwtv5.Token) (any, error) {
			return []byte(ac.AccessTokenSecret), nil
		},
		kratosjwt.WithSigningMethod(jwtv5.SigningMethodHS256),
		kratosjwt.WithClaims(func() jwtv5.Claims {
			return &biz.GameClaims{}
		}),
	)

	opts := []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			selector.Server(
				jwtMiddleware,
				custommid.InjectClaims(),
				custommid.CheckBlacklist(authUc),
			).
				Match(func(ctx context.Context, operation string) bool {
					return !publicOps[operation]
				}).
				Build(),
		),
	}

	if c.Http.Network != "" {
		opts = append(opts, http.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := http.NewServer(opts...)
	userv1.RegisterUserHTTPServer(srv, user)
	return srv
}
