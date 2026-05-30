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
		return fmt.Errorf("marshal session: %w", err)
	}

	if err := r.rdb.Set(ctx, key, data, gameSessionTTL).Err(); err != nil {
		return fmt.Errorf("save session to redis: %w", err)
	}

	return nil
}

func (r *GameRepo) LoadSession(ctx context.Context, roomID string) (*biz.GameSession, error) {
	key := gameSessionKeyPrefix + roomID

	data, err := r.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("load session from redis: %w", err)
	}

	var session biz.GameSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

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
