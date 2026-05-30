package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type MatchRepo interface {
	JoinPool(ctx context.Context, userID int64) error
	CancelMatch(ctx context.Context, userID int64) error
	GetPlayerStatus(ctx context.Context, userID int64) (status string, roomID string, err error)

	PopMatchedPlayers(ctx context.Context, requiredPlayers int) ([]int64, error)
	PushBackToPool(ctx context.Context, users []int64) error
	CreateRoomAndUpdateState(ctx context.Context, roomID string, users []int64) error
	ClearPlayerState(ctx context.Context, userID int64) error
	DeleteRoomInfo(ctx context.Context, roomID string) error
}

type MatchUsecase struct {
	repo MatchRepo
	log  *log.Helper
}

func NewMatchUsecase(matchRepo MatchRepo, logger log.Logger) *MatchUsecase {
	return &MatchUsecase{
		repo: matchRepo,
		log:  log.NewHelper(logger),
	}
}

func (uc *MatchUsecase) LockAndMatch(ctx context.Context, requiredPlayers int) {
	matchedUsers, err := uc.repo.PopMatchedPlayers(ctx, requiredPlayers)
	if err != nil {
		uc.log.WithContext(ctx).Errorf("pull match pool failed: %v", err)
		return
	}

	if len(matchedUsers) == 0 {
		return
	}

	roomID := generateRoomID()

	err = uc.repo.CreateRoomAndUpdateState(ctx, roomID, matchedUsers)
	if err != nil {
		uc.log.WithContext(ctx).Errorf("create room failed: %v", err)
		if pushErr := uc.repo.PushBackToPool(ctx, matchedUsers); pushErr != nil {
			uc.log.WithContext(ctx).Errorf("disaster recovery failed, push back to pool: %v, lost players: %v", pushErr, matchedUsers)
		}
		return
	}

	uc.log.WithContext(ctx).Infof("room [%s] created, players: %v", roomID, matchedUsers)
}

func generateRoomID() string {
	return uuid.New().String()
}
