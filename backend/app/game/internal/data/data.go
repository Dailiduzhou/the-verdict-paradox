package data

import (
	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
)

}
// ProviderSet is data providers.
var ProviderSet = wire.NewSet(
	NewPgxPool,  NewData, NewRedisClient, NewAuthRepo, NewUserRepo,
	wire.Bind(new(biz.AuthRepo), new(*AuthRepo)),
	wire.Bind(new(biz.UserRepo), new(*UserRepo)),
)

// Data .
type Data struct {
	pool        *pgxpool.Pool
	rdb         *redis.Client
	q           *db.Queries
	sg          *singleflight.Group
}

// NewData .
func NewData(c *conf.Data, pool *pgxpool.Pool, rdb *redis.Client) (*Data, func(), error) {
	ctx := context.Background()

	cleanup := func() {
		rdb.Close()
		pool.Close()

		log.Info("closing the data resources")
	}
	return &Data{
		pool:        pool,
		rdb:         rdb,
		q:           db.New(pool),
		sg:          &singleflight.Group{},
	}, cleanup, nil
}

func NewRedisClient(c *conf.Data) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     c.Redis.Addr,
		Password: "",
		DB:       0,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		rdb.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return rdb, nil
}

func NewPgxPool(c *conf.Data) (*pgxpool.Pool, func(), error) {
	pool, err := pgxpool.New(context.Background(), c.Database.Source)
	if err != nil {
		return nil, nil, fmt.Errorf("create pgx pool: %w", err)
	}
	cleanup := func() { pool.Close() }
	return pool, cleanup, nil
}

