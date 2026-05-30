//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/biz"
	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/conf"
	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/data"
	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/server"
	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/service"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// wireApp init kratos application.
func wireApp(*conf.Server, *conf.Data, *conf.Auth, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(server.ProviderSet, data.ProviderSet, biz.ProviderSet, service.ProviderSet, newApp))
}
