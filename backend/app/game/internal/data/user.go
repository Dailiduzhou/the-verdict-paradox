package data

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/biz"
	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/data/db"
	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/utils/pwdhash"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
)

var _ biz.UserRepo = (*UserRepo)(nil)

type UserRepo struct {
	data *Data
	log  *log.Helper
}

func NewUserRepo(data *Data, logger log.Logger) *UserRepo {
	return &UserRepo{data: data, log: log.NewHelper(logger)}
}

func toBizUser(u db.User) *biz.User {
	return &biz.User{
		ID:           u.ID,
		Name:         u.Name,
		PasswordHash: u.PasswordHash,
		CreatedAt:    u.CreatedAt.Time,
		UpdatedAt:    u.UpdatedAt.Time,
	}
}

func (r *UserRepo) getCache(ctx context.Context, key string) (*biz.User, error) {
	val, err := r.data.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var user biz.User
	if err := json.Unmarshal(val, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) setCache(ctx context.Context, key string, user *biz.User) {
	data, err := json.Marshal(user)
	if err != nil {
		r.log.WithContext(ctx).Errorf("marshal user cache: %v", err)
		return
	}
	jitter := time.Duration(rand.Intn(10)) * time.Minute
	exp := jitter + 10*time.Minute
	r.data.rdb.Set(ctx, key, data, exp)
}

func (r *UserRepo) deleteCache(ctx context.Context, id int64) {
	cacheKey := fmt.Sprintf("user:%d", id)
	r.data.rdb.Del(ctx, cacheKey)
}

func (r *UserRepo) CreateUser(ctx context.Context, name, password string) (*biz.User, error) {
	hashedPwd, err := pwdhash.HashPassword(password)
	if err != nil {
		r.log.WithContext(ctx).Errorf("Password hash failed:%v", err)
		return nil, errors.InternalServer("PASSWORD_ERROR", "password hash failed")
	}
	user, err := r.data.q.CreateUser(ctx, db.CreateUserParams{
		Name:         name,
		PasswordHash: hashedPwd,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if stderrors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, errors.Conflict("USER_EXISTS", fmt.Sprintf("user %s already exists", name))
		}
		return nil, errors.InternalServer("DB_ERROR", "failed to create user")
	}

	return toBizUser(user), nil
}

func (r *UserRepo) GetUserByID(ctx context.Context, id int64) (*biz.User, error) {
	cacheKey := fmt.Sprintf("user:%d", id)

	user, err := r.getCache(ctx, cacheKey)
	if err == nil {
		return user, nil
	}
	if !stderrors.Is(err, redis.Nil) {
		r.log.WithContext(ctx).Errorf("Error finding user cache:%v", err)
	}

	sfKey := fmt.Sprintf("sf:user:%d", id)
	val, err, _ := r.data.sg.Do(sfKey, func() (interface{}, error) {
		userDoublecheck, err := r.getCache(ctx, cacheKey)
		if err == nil {
			return userDoublecheck, nil
		}

		r.log.WithContext(ctx).Debugf("user %d cache miss, fetching from DB", id)
		dbUser, err := r.data.q.GetUserByID(ctx, id)
		if err != nil {
			if stderrors.Is(err, pgx.ErrNoRows) {
				return nil, errors.NotFound("USER_NOT_FOUND", fmt.Sprintf("user %d not found", id))
			}
			return nil, errors.InternalServer("DB_ERROR", "failed to fetch user")
		}
		finaldbUser := toBizUser(dbUser)
		r.setCache(ctx, cacheKey, finaldbUser)
		return finaldbUser, nil
	})

	if err != nil {
		return nil, err
	}

	return val.(*biz.User), nil
}

func (r *UserRepo) GetUserByName(ctx context.Context, name string) (*biz.User, error) {
	dbUser, err := r.data.q.GetUserByName(ctx, name)
	if err != nil {
		if stderrors.Is(err, pgx.ErrNoRows) {
			return nil, errors.NotFound("USER_NOT_FOUND", fmt.Sprintf("user %s not found", name))
		}
		return nil, errors.InternalServer("DB_ERROR", "failed to fetch user")
	}
	return toBizUser(dbUser), nil
}

func (r *UserRepo) UpdateUser(ctx context.Context, id int64, name string) (*biz.User, error) {
	dbUser, err := r.data.q.UpdateUserInfo(ctx, db.UpdateUserInfoParams{
		ID:   id,
		Name: name,
	})
	if err != nil {
		if stderrors.Is(err, pgx.ErrNoRows) {
			return nil, errors.NotFound("USER_NOT_FOUND", fmt.Sprintf("user %d not found", id))
		}
		return nil, errors.InternalServer("DB_ERROR", "failed to update user")
	}
	u := toBizUser(dbUser)
	r.deleteCache(ctx, id)
	return u, nil
}

func (r *UserRepo) DeleteUser(ctx context.Context, id int64) error {
	if err := r.data.q.DeleteUser(ctx, id); err != nil {
		return errors.InternalServer("DB_ERROR", "failed to delete user")
	}
	r.deleteCache(ctx, id)
	return nil
}
