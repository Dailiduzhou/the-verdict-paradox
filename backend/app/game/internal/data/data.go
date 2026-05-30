package data

import (
	"context"
	"fmt"

	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/biz"
	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/conf"
	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/data/db"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(
	NewPgxPool, NewData, NewRedisClient, NewAuthRepo, NewUserRepo, NewMatchRepo,
	wire.Bind(new(biz.AuthRepo), new(*AuthRepo)),
	wire.Bind(new(biz.UserRepo), new(*UserRepo)),
	wire.Bind(new(biz.MatchRepo), new(*MatchRepo)),
)

// Data .
type Data struct {
	rdb *redis.Client
	q   *db.Queries
	sg  *singleflight.Group
}

// NewData .
func NewData(pool *pgxpool.Pool, rdb *redis.Client) (*Data, func(), error) {
	cleanup := func() {
		log.Info("closing the data resources")
	}
	return &Data{
		rdb: rdb,
		q:   db.New(pool),
		sg:  &singleflight.Group{},
	}, cleanup, nil
}

func NewRedisClient(c *conf.Data) (*redis.Client, func(), error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     c.Redis.Addr,
		Password: "",
		DB:       0,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		rdb.Close()
		return nil, nil, fmt.Errorf("ping redis: %w", err)
	}

	cleanup := func() { rdb.Close() }
	return rdb, cleanup, nil
}

func NewPgxPool(c *conf.Data) (*pgxpool.Pool, func(), error) {
	pool, err := pgxpool.New(context.Background(), c.Database.Source)
	if err != nil {
		return nil, nil, fmt.Errorf("create pgx pool: %w", err)
	}
	cleanup := func() { pool.Close() }
	return pool, cleanup, nil
}
