package service

import (
	"context"

	pb "github.com/Dailiduzhou/the-verdict-paradox/backend/api/user/v1"
	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/biz"
	"github.com/go-kratos/kratos/v2/log"
)

type UserService struct {
	pb.UnimplementedUserServer
	authUc   *biz.AuthUsecase
	userRepo biz.UserRepo
	log      *log.Helper
}

func NewUserService(authUc *biz.AuthUsecase, userRepo biz.UserRepo, logger log.Logger) *UserService {
	return &UserService{
		authUc:   authUc,
		userRepo: userRepo,
		log:      log.NewHelper(logger),
	}
}

func (s *UserService) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterReply, error) {
	return &pb.RegisterReply{}, nil
}

func (s *UserService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginReply, error) {
	return &pb.LoginReply{}, nil
}

func (s *UserService) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.UserInfo, error) {
	return &pb.UserInfo{}, nil
}

func (s *UserService) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UserInfo, error) {
	return &pb.UserInfo{}, nil
}

func (s *UserService) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*pb.DeleteUserReply, error) {
	return &pb.DeleteUserReply{}, nil
}

func (s *UserService) RefreshToken(ctx context.Context, req *pb.RefreshRequest) (*pb.RefreshReply, error) {
	return &pb.RefreshReply{}, nil
}
