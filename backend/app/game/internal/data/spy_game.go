package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/biz"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
)

var _ biz.GameRepo = (*GameRepo)(nil)

const gameSessionKeyPrefix = "game:session:"
const gameSessionTTL = 2 * time.Hour

type GameRepo struct {
	rdb *redis.Client
	log *log.Helper
}

func NewGameRepo(rdb *redis.Client, logger log.Logger) *GameRepo {
	return &GameRepo{rdb: rdb, log: log.NewHelper(logger)}
}

func (r *GameRepo) SaveSession(ctx context.Context, session *biz.GameSession) error {
	key := gameSessionKeyPrefix + session.RoomID

	data, err := json.Marshal(session)
	if err != nil {
		r.log.WithContext(ctx).Errorf("marshal session [%s] failed: %v", session.RoomID, err)
		return fmt.Errorf("marshal session: %w", err)
	}

	if err := r.rdb.Set(ctx, key, data, gameSessionTTL).Err(); err != nil {
		r.log.WithContext(ctx).Errorf("save session [%s] to redis failed: %v", session.RoomID, err)
		return fmt.Errorf("save session to redis: %w", err)
	}

	r.log.WithContext(ctx).Debugf("session [%s] saved, round=%d, phase=%s", session.RoomID, session.Round, session.Phase)
	return nil
}

func (r *GameRepo) LoadSession(ctx context.Context, roomID string) (*biz.GameSession, error) {
	key := gameSessionKeyPrefix + roomID

	data, err := r.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			r.log.WithContext(ctx).Debugf("session [%s] not found in redis", roomID)
			return nil, nil
		}
		r.log.WithContext(ctx).Errorf("load session [%s] from redis failed: %v", roomID, err)
		return nil, fmt.Errorf("load session from redis: %w", err)
	}

	var session biz.GameSession
	if err := json.Unmarshal(data, &session); err != nil {
		r.log.WithContext(ctx).Errorf("unmarshal session [%s] failed: %v", roomID, err)
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	r.log.WithContext(ctx).Infof("session [%s] loaded from redis, round=%d, phase=%s", roomID, session.Round, session.Phase)

	session.AnswerCount = 0
	session.VoteCount = 0
	session.PendingAI = false
	session.StopCh = nil
	if session.UsedQuestions == nil {
		session.UsedQuestions = make(map[int]bool)
	}

	return &session, nil
}

func (r *GameRepo) DeleteSession(ctx context.Context, roomID string) error {
	key := gameSessionKeyPrefix + roomID
	return r.rdb.Del(ctx, key).Err()
}
