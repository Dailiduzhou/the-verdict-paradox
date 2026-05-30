package service

import (
	"context"

	pb "github.com/Dailiduzhou/the-verdict-paradox/backend/api/user/v1"
	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/biz"
	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type UserService struct {
	pb.UnimplementedUserServer
	authUc *biz.AuthUsecase
	userUc *biz.UserUsecase
	log    *log.Helper
}

func NewUserService(authUc *biz.AuthUsecase, userUc *biz.UserUsecase, logger log.Logger) *UserService {
	return &UserService{
		authUc: authUc,
		userUc: userUc,
		log:    log.NewHelper(logger),
	}
}

func (s *UserService) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterReply, error) {
	user, err := s.userUc.Register(ctx, req.Name, req.Password)
	if err != nil {
		return nil, err
	}
	return &pb.RegisterReply{Id: user.ID}, nil
}

func (s *UserService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginReply, error) {
	user, token, err := s.authUc.Login(ctx, req.Name, req.Password)
	if err != nil {
		return nil, err
	}
	return &pb.LoginReply{Id: user.ID, Token: token}, nil
}

func (s *UserService) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.UserInfo, error) {
	user, err := s.userUc.GetUser(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &pb.UserInfo{
		Id:        user.ID,
		Name:      user.Name,
		CreatedAt: timestamppb.New(user.CreatedAt),
	}, nil
}

func (s *UserService) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UserInfo, error) {
	user, err := s.userUc.UpdateUser(ctx, req.Id, req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.UserInfo{
		Id:        user.ID,
		Name:      user.Name,
		CreatedAt: timestamppb.New(user.CreatedAt),
	}, nil
}

func (s *UserService) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*pb.DeleteUserReply, error) {
	if err := s.userUc.DeleteUser(ctx, req.Id); err != nil {
		return nil, err
	}
	return &pb.DeleteUserReply{}, nil
}

func (s *UserService) RefreshToken(ctx context.Context, req *pb.RefreshRequest) (*pb.RefreshReply, error) {
	accessToken, refreshToken, err := s.authUc.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, err
	}
	return &pb.RefreshReply{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *UserService) Verify(ctx context.Context, req *pb.VerifyRequest) (*pb.VerifyReply, error) {
	_, err := s.authUc.VerifyToken(ctx, req.Token)
	if err != nil {
		return &pb.VerifyReply{Valid: false}, nil
	}
	return &pb.VerifyReply{Valid: true}, nil
}
