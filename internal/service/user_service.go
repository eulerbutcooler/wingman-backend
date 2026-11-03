package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Amanyd/backend/internal/config"
	"github.com/Amanyd/backend/internal/domain"
	"github.com/Amanyd/backend/internal/port"
	"github.com/Amanyd/backend/pkg/jwt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	users  port.UserRepository
	jwtCfg config.JWTConfig
}

func NewUserService(users port.UserRepository, jwtCfg config.JWTConfig) *UserService {
	return &UserService{users: users, jwtCfg: jwtCfg}
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (s *UserService) Register(ctx context.Context, name, enrollmentID, rank, password string, role domain.Role) (*domain.User, error) {
	name = strings.TrimSpace(name)
	enrollmentID = strings.TrimSpace(enrollmentID)
	rank = strings.TrimSpace(rank)

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &domain.User{
		Name:         name,
		EnrollmentID: enrollmentID,
		Rank:         rank,
		Role:         role,
		PasswordHash: string(hash),
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) Login(ctx context.Context, enrollmentID, password string) (*TokenPair, error) {
	enrollmentID = strings.TrimSpace(enrollmentID)

	user, err := s.users.GetByEnrollmentID(ctx, enrollmentID)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, domain.ErrUnauthorized
	}

	return s.generateTokenPair(user)
}

func (s *UserService) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	claims, err := jwt.ValidateToken(refreshToken, s.jwtCfg.JWTRefreshSecret)
	if err != nil {
		return "", domain.ErrUnauthorized
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return "", domain.ErrUnauthorized
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return "", domain.ErrUnauthorized
	}

	access, err := jwt.GenerateAccessToken(
		user.ID.String(), string(user.Role), user.Rank,
		s.jwtCfg.JWTAccessSecret, s.jwtCfg.JWTAccessTTL,
	)
	if err != nil {
		return "", fmt.Errorf("generate access token: %w", err)
	}
	return access, nil
}

func (s *UserService) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return s.users.GetByID(ctx, userID)
}

func (s *UserService) generateTokenPair(user *domain.User) (*TokenPair, error) {
	access, err := jwt.GenerateAccessToken(
		user.ID.String(), string(user.Role), user.Rank,
		s.jwtCfg.JWTAccessSecret, s.jwtCfg.JWTAccessTTL,
	)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refresh, err := jwt.GenerateRefreshToken(
		user.ID.String(),
		s.jwtCfg.JWTRefreshSecret, s.jwtCfg.JWTRefreshTTL,
	)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	return &TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}
