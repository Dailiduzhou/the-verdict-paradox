package data

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/biz"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
)

var _ biz.MatchRepo = (*MatchRepo)(nil)

type MatchRepo struct {
	rdb *redis.Client
	log *log.Helper
}

func NewMatchRepo(rdb *redis.Client, logger log.Logger) *MatchRepo {
	return &MatchRepo{rdb: rdb, log: log.NewHelper(logger)}
}

const (
	MatchPoolKey       = "match:pool:6p"
	UserStateKeyPrefix = "user:state:"
)

func (r *MatchRepo) JoinPool(ctx context.Context, userID int64) error {
	now := float64(time.Now().UnixNano())
	uidStr := strconv.FormatInt(userID, 10)
	stateKey := UserStateKeyPrefix + uidStr

	pipe := r.rdb.TxPipeline()
	pipe.Set(ctx, stateKey, "MATCHING", 10*time.Minute)
	pipe.ZAdd(ctx, MatchPoolKey, redis.Z{Score: now, Member: uidStr})

	_, err := pipe.Exec(ctx)
	return err
}

func (r *MatchRepo) CancelMatch(ctx context.Context, userID int64) error {
	uidStr := strconv.FormatInt(userID, 10)
	script := redis.NewScript(`
		local state_key = KEYS[1]
		local pool_key = KEYS[2]
		local user_id = ARGV[1]

		local state = redis.call('GET', state_key)
		if state == 'MATCHING' then
			redis.call('DEL', state_key)
			redis.call('ZREM', pool_key, user_id)
			return 1
		end
		return 0
	`)

	stateKey := UserStateKeyPrefix + uidStr
	res, err := script.Run(ctx, r.rdb, []string{stateKey, MatchPoolKey}, uidStr).Int()
	if err != nil {
		return err
	}
	if res == 0 {
		return errors.Conflict("CONFLICT", "maybe in game")
	}
	return nil
}

func (r *MatchRepo) PopMatchedPlayers(ctx context.Context, requiredCount int) ([]int64, error) {
	script := redis.NewScript(`
		local pool_key = KEYS[1]
		local required = tonumber(ARGV[1])

		local count = redis.call('ZCARD', pool_key)
		if count >= required then
			local result = redis.call('ZPOPMIN', pool_key, required)
			local members = {}
			for i=1, #result, 2 do
				table.insert(members, result[i])
			end
			return members
		else
			return {}
		end
	`)

	members, err := script.Run(ctx, r.rdb, []string{MatchPoolKey}, requiredCount).StringSlice()
	if err != nil && err != redis.Nil {
		return nil, err
	}

	userIDs := make([]int64, 0, len(members))
	for _, m := range members {
		id, err := strconv.ParseInt(m, 10, 64)
		if err != nil {
			r.log.WithContext(ctx).Errorf("parse user id from pool: %v", err)
			continue
		}
		userIDs = append(userIDs, id)
	}
	return userIDs, nil
}

func (r *MatchRepo) GetPlayerStatus(ctx context.Context, userID int64) (string, string, error) {
	uidStr := strconv.FormatInt(userID, 10)
	stateKey := UserStateKeyPrefix + uidStr
	state, err := r.rdb.Get(ctx, stateKey).Result()
	if err != nil {
		if err == redis.Nil {
			return "IDLE", "", nil
		}
		return "", "", errors.InternalServer("MATCH_ERROR", "failed to get player status")
	}

	if state == "MATCHING" {
		return "MATCHING", "", nil
	}

	roomID := ""
	if len(state) > len("IN_GAME_") {
		roomID = state[len("IN_GAME_"):]
	}
	return "IN_GAME", roomID, nil
}

func (r *MatchRepo) PushBackToPool(ctx context.Context, users []int64) error {
	if len(users) == 0 {
		return nil
	}

	now := float64(time.Now().UnixMilli())
	members := make([]redis.Z, 0, len(users))
	for _, uid := range users {
		members = append(members, redis.Z{
			Score:  now,
			Member: strconv.FormatInt(uid, 10),
		})
	}
	if err := r.rdb.ZAdd(ctx, MatchPoolKey, members...).Err(); err != nil {
		r.log.WithContext(ctx).Errorf("PushBackToPool ZAdd failed: %v", err)
		return errors.InternalServer("MATCH_ERROR", "failed to push back players")
	}
	r.log.WithContext(ctx).Infof("匹配池回退成功: %v", users)
	return nil
}

func (r *MatchRepo) CreateRoomAndUpdateState(ctx context.Context, roomID string, userIDs []int64) error {
	pipe := r.rdb.TxPipeline()
	roomKey := "room:info:" + roomID

	for _, uid := range userIDs {
		uidStr := strconv.FormatInt(uid, 10)
		stateVal := fmt.Sprintf("IN_GAME_%s", roomID)
		pipe.Set(ctx, UserStateKeyPrefix+uidStr, stateVal, 2*time.Hour)
		pipe.SAdd(ctx, roomKey, uidStr)
	}

	pipe.Expire(ctx, roomKey, 2*time.Hour)

	_, err := pipe.Exec(ctx)
	return err
}

func (r *MatchRepo) ClearPlayerState(ctx context.Context, userID int64) error {
	uidStr := strconv.FormatInt(userID, 10)
	return r.rdb.Del(ctx, UserStateKeyPrefix+uidStr).Err()
}

func (r *MatchRepo) DeleteRoomInfo(ctx context.Context, roomID string) error {
	return r.rdb.Del(ctx, "room:info:"+roomID).Err()
}
