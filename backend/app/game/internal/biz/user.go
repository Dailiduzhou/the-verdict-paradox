package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/conf"
	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/utils/pwdhash"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/golang-jwt/jwt/v5"
)

type User struct {
	ID           int64
	Name         string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserRepo interface {
	CreateUser(ctx context.Context, name, password string) (*User, error)
	GetUserByID(ctx context.Context, id int64) (*User, error)
	GetUserByName(ctx context.Context, name string) (*User, error)
	UpdateUser(ctx context.Context, id int64, name string) (*User, error)
	DeleteUser(ctx context.Context, id int64) error
}

type UserUsecase struct {
	userRepo UserRepo
	log      *log.Helper
}

func NewUserUsecase(userRepo UserRepo, logger log.Logger) *UserUsecase {
	return &UserUsecase{
		userRepo: userRepo,
		log:      log.NewHelper(logger),
	}
}

func (uc *UserUsecase) Register(ctx context.Context, name, password string) (*User, error) {
	return uc.userRepo.CreateUser(ctx, name, password)
}

func (uc *UserUsecase) GetUser(ctx context.Context, id int64) (*User, error) {
	return uc.userRepo.GetUserByID(ctx, id)
}

func (uc *UserUsecase) GetUserByName(ctx context.Context, name string) (*User, error) {
	return uc.userRepo.GetUserByName(ctx, name)
}

func (uc *UserUsecase) UpdateUser(ctx context.Context, id int64, name string) (*User, error) {
	return uc.userRepo.UpdateUser(ctx, id, name)
}

func (uc *UserUsecase) DeleteUser(ctx context.Context, id int64) error {
	return uc.userRepo.DeleteUser(ctx, id)
}

type AuthRepo interface {
	SetBlacklist(ctx context.Context, tokenID string, expiration time.Duration) error
	IsBlacklisted(ctx context.Context, tokenID string) (bool, error)
}

type GameClaims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

type AuthUsecase struct {
	userRepo       UserRepo
	authRepo       AuthRepo
	accessSecret   string
	accessTimeout  time.Duration
	refreshSecret  string
	refreshTimeout time.Duration
}

func NewAuthUsecase(userRepo UserRepo, authRepo AuthRepo, ac *conf.Auth) *AuthUsecase {
	return &AuthUsecase{
		userRepo:       userRepo,
		authRepo:       authRepo,
		accessSecret:   ac.AccessTokenSecret,
		accessTimeout:  ac.AccessTokenTimeout.AsDuration(),
		refreshSecret:  ac.RefreshTokenSecret,
		refreshTimeout: ac.RefreshTokenTimeout.AsDuration(),
	}
}

func (uc *AuthUsecase) Login(ctx context.Context, name, password string) (*User, string, error) {
	u, err := uc.userRepo.GetUserByName(ctx, name)
	if err != nil {
		return nil, "", errors.Unauthorized("AUTH_ERROR", "invalid username or password")
	}
	if err := pwdhash.ComparePassword(u.PasswordHash, password); err != nil {
		return nil, "", errors.Unauthorized("AUTH_ERROR", "invalid username or password")
	}
	token, err := uc.GenerateAccessToken(u.ID)
	if err != nil {
		return nil, "", errors.InternalServer("TOKEN_ERROR", "failed to generate token")
	}
	return u, token, nil
}

func (uc *AuthUsecase) RefreshToken(ctx context.Context, refreshTokenStr string) (string, string, error) {
	claims, err := uc.ParseRefreshToken(refreshTokenStr)
	if err != nil {
		return "", "", errors.Unauthorized("AUTH_ERROR", "invalid refresh token")
	}
	blacklisted, err := uc.IsTokenBlacklisted(ctx, claims.ID)
	if err != nil {
		return "", "", errors.InternalServer("AUTH_ERROR", "failed to check token")
	}
	if blacklisted {
		return "", "", errors.Unauthorized("AUTH_ERROR", "token has been revoked")
	}
	exp := claims.ExpiresAt.Time
	if err := uc.BlacklistToken(ctx, claims.ID, exp); err != nil {
		return "", "", errors.InternalServer("AUTH_ERROR", "failed to revoke old token")
	}
	newAccess, err := uc.GenerateAccessToken(claims.UserID)
	if err != nil {
		return "", "", errors.InternalServer("TOKEN_ERROR", "failed to generate token")
	}
	newRefresh, err := uc.GenerateRefreshToken(claims.UserID)
	if err != nil {
		return "", "", errors.InternalServer("TOKEN_ERROR", "failed to generate token")
	}
	return newAccess, newRefresh, nil
}

func (uc *AuthUsecase) GenerateAccessToken(userID int64) (string, error) {
	now := time.Now()
	tokenID := generateTokenID()
	claims := GameClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(uc.accessTimeout)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(uc.accessSecret))
}

func (uc *AuthUsecase) GenerateRefreshToken(userID int64) (string, error) {
	now := time.Now()
	tokenID := generateTokenID()
	claims := GameClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(uc.refreshTimeout)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(uc.refreshSecret))
}

func (uc *AuthUsecase) ParseAccessToken(tokenStr string) (*GameClaims, error) {
	return uc.parseToken(tokenStr, uc.accessSecret)
}

func (uc *AuthUsecase) ParseRefreshToken(tokenStr string) (*GameClaims, error) {
	return uc.parseToken(tokenStr, uc.refreshSecret)
}

func (uc *AuthUsecase) BlacklistToken(ctx context.Context, tokenID string, expiresAt time.Time) error {
	expiration := time.Until(expiresAt)
	if expiration <= 0 {
		return nil
	}
	return uc.authRepo.SetBlacklist(ctx, tokenID, expiration)
}

func (uc *AuthUsecase) IsTokenBlacklisted(ctx context.Context, tokenID string) (bool, error) {
	return uc.authRepo.IsBlacklisted(ctx, tokenID)
}

func (uc *AuthUsecase) parseToken(tokenStr, secret string) (*GameClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &GameClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*GameClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

func generateTokenID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
