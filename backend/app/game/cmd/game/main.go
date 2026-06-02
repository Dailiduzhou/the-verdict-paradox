package main

import (
	"flag"
	"os"

	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/conf"
	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/server"
	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/utils/logger"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"

	_ "go.uber.org/automaxprocs"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	// Name is the name of the compiled software.
	Name = "the-verdict-paradox-backend"
	// Version is the version of the compiled software.
	Version string
	// flagconf is the config flag.
	flagconf string

	id, _ = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf config.yaml")

	if p := findEnvFile(); p != "" {
		if err := loadEnv(p); err != nil {
			log.Warnf("failed to load .env from %s: %v", p, err)
		}
	}
}

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server, ms *server.MatchServer) *kratos.App {
	return kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			gs,
			hs,
			ms,
		),
	)
}

func main() {
	flag.Parse()

	logger := logger.NewJSONLogger()
	log.SetLogger(logger)
	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
		),
	)
	defer c.Close()

	if err := c.Load(); err != nil {
		panic(err)
	}

	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		panic(err)
	}

	log.Infof("starting %s version=%s id=%s", Name, Version, id)
	log.Infof("config loaded from %s", flagconf)

	app, cleanup, err := wireApp(bc.Server, bc.Data, bc.Auth, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	if err := app.Run(); err != nil {
		panic(err)
	}
}
