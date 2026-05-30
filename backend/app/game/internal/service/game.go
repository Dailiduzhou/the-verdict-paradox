package service

import (
	"context"
	"fmt"

	pb "github.com/Dailiduzhou/the-verdict-paradox/backend/api/game/v1"
	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/biz"
	"github.com/go-kratos/kratos/v2/log"
)

type GameService struct {
	pb.UnimplementedGameServer
	userRepo  biz.UserRepo
	matchRepo biz.MatchRepo
	gameRepo  biz.GameRepo
	log       *log.Helper
}

func NewGameService(userRepo biz.UserRepo, matchRepo biz.MatchRepo, gameRepo biz.GameRepo, logger log.Logger) *GameService {
	return &GameService{
		userRepo:  userRepo,
		matchRepo: matchRepo,
		gameRepo:  gameRepo,
		log:       log.NewHelper(logger),
	}
}

func (s *GameService) StartMatch(ctx context.Context, req *pb.StartMatchRequest) (*pb.StartMatchReply, error) {
	user, err := s.userRepo.GetUserByName(ctx, req.Name)
	if err != nil {
		s.log.WithContext(ctx).Errorf("user not found: %s", req.Name)
		return nil, err
	}

	if err := s.matchRepo.JoinPool(ctx, user.ID); err != nil {
		s.log.WithContext(ctx).Errorf("join pool failed: %v", err)
		return nil, err
	}

	matchID := fmt.Sprintf("%d", user.ID)
	s.log.WithContext(ctx).Infof("玩家 %s (ID:%d) 加入匹配池", req.Name, user.ID)
	return &pb.StartMatchReply{MatchID: matchID}, nil
}

func (s *GameService) CancelMatch(ctx context.Context, req *pb.CancelMatchRequest) (*pb.CancelMatchReply, error) {
	user, err := s.userRepo.GetUserByName(ctx, req.Name)
	if err != nil {
		s.log.WithContext(ctx).Errorf("user not found: %s", req.Name)
		return nil, err
	}

	if err := s.matchRepo.CancelMatch(ctx, user.ID); err != nil {
		s.log.WithContext(ctx).Errorf("cancel match failed: %v", err)
		return nil, err
	}

	s.log.WithContext(ctx).Infof("玩家 %s (ID:%d) 取消匹配", req.Name, user.ID)
	return &pb.CancelMatchReply{}, nil
}

func (s *GameService) GetMatchStatus(ctx context.Context, req *pb.GetMatchStatusRequest) (*pb.GetMatchStatusReply, error) {
	user, err := s.userRepo.GetUserByName(ctx, req.Matchid)
	if err != nil {
		s.log.WithContext(ctx).Errorf("user not found: %s", req.Matchid)
		return nil, err
	}

	status, roomID, err := s.matchRepo.GetPlayerStatus(ctx, user.ID)
	if err != nil {
		s.log.WithContext(ctx).Errorf("get status failed: %v", err)
		return nil, err
	}

	reply := &pb.GetMatchStatusReply{
		Status: status,
		RoomID: roomID,
	}

	if status == "IN_GAME" && roomID != "" {
		session, err := s.gameRepo.LoadSession(ctx, roomID)
		if err != nil {
			s.log.WithContext(ctx).Errorf("load game session failed: %v", err)
		} else if session != nil {
			for _, p := range session.Players {
				reply.Users = append(reply.Users, p.Name)
			}
		}
	}

	return reply, nil
}
