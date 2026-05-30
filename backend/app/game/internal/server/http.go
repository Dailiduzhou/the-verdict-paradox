package server

import (
	"context"
	stdhttp "net/http"
	"strconv"
	"strings"

	docs "github.com/Dailiduzhou/the-verdict-paradox/backend/api/docs"
	gamev1 "github.com/Dailiduzhou/the-verdict-paradox/backend/api/game/v1"
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
func NewHTTPServer(c *conf.Server, user *service.UserService, game *service.GameService, authUc *biz.AuthUsecase, ac *conf.Auth, rm *biz.RoomManager, logger log.Logger) *http.Server {
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
		http.Filter(custommid.CORS()),
		http.Middleware(
			recovery.Recovery(),
			selector.Server(
				jwtMiddleware,
				custommid.InjectClaims(),
				custommid.CheckBlacklist(authUc),
			).
				Match(func(ctx context.Context, operation string) bool {
					if len(operation) >= 3 && operation[:3] == "/ws" {
						return false
					}
					if strings.HasPrefix(operation, "/docs") {
						return false
					}
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
	gamev1.RegisterGameHTTPServer(srv, game)

	srv.HandlePrefix("/docs/", docs.Handler())

	srv.HandleFunc("/ws/room/{room_id}", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		roomID := r.PathValue("room_id")

		token := r.URL.Query().Get("token")
		if token == "" {
			stdhttp.Error(w, `{"code":401,"reason":"AUTH_ERROR","message":"missing token"}`, stdhttp.StatusUnauthorized)
			return
		}

		claims, err := authUc.VerifyToken(r.Context(), token)
		if err != nil {
			stdhttp.Error(w, "", stdhttp.StatusUnauthorized)
			return
		}

		userName, err := user.GetUserName(r.Context(), claims.UserID)
		if err != nil {
			stdhttp.Error(w, "", stdhttp.StatusUnauthorized)
			return
		}

		_ = rm.HandleWS(w, r, roomID, strconv.FormatInt(claims.UserID, 10), userName)
	})

	return srv
}
