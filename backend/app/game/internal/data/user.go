package data

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/biz"
	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/data/db"
	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/utils"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
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

func (r *UserRepo) CreateUser(ctx context.Context, name, password string) (*biz.User, error) {
	hashedPwd, err := utils.HashPassword(password)
	if err != nil {
		r.log.WithContext(ctx).Errorf("Password hash failed:%v", err)
		return nil, errors.InternalServer("Password hash failed:%v", err)
	}
	user, err := r.data.q.CreateUser(ctx, &db.CreateUserParams{
		Name:         name,
		PasswordHash: hashedPwd,
	})
	if err != nil {
		return nil, errors.InternalServer("DB ERROR", "fail to create user")
	}

	return toBizUser(user), nil
}

func (r *UserRepo) GetUserByID(ctx context.Context, id int64) (*biz.User, error) {
	cacheKey := fmt.Sprintf("user:%d", id)
}
