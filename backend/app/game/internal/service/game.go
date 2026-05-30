package service

import (
	"context"

	pb "github.com/Dailiduzhou/the-verdict-paradox/backend/api/game/v1"
)

type GameService struct {
	pb.UnimplementedGameServer
}

func NewGameService() *GameService {
	return &GameService{}
}

func (s *GameService) StartMatch(ctx context.Context, req *pb.StartMatchRequest) (*pb.StartMatchReply, error) {
    return &pb.StartMatchReply{}, nil
}
func (s *GameService) CancelMatch(ctx context.Context, req *pb.CancelMatchRequest) (*pb.CancelMatchReply, error) {
    return &pb.CancelMatchReply{}, nil
}
func (s *GameService) GetMatchStatus(ctx context.Context, req *pb.GetMatchStatusRequest) (*pb.GetMatchStatusReply, error) {
    return &pb.GetMatchStatusReply{}, nil
}
