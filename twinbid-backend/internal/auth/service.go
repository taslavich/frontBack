package auth

import (
	"context"

	"twinbid-backend/internal/config"
	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
)

type Service struct {
	repo *Repository
	cfg  config.JWTConfig
}

func NewService(repo *Repository, cfg config.JWTConfig) *Service {
	return &Service{repo: repo, cfg: cfg}
}

type AuthResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	User         models.User `json:"user"`
}

type AuthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (s *Service) Signup(ctx context.Context, email, password, fullName, managerTelegram string) (AuthResponse, error) {
	if email == "" || password == "" || managerTelegram == "" {
		return AuthResponse{}, httpx.BadRequest("email, password and manager_telegram are required")
	}
	u, err := s.repo.CreateUser(ctx, email, password, fullName, managerTelegram)
	if err != nil {
		return AuthResponse{}, err
	}
	tokens, err := s.issueTokens(ctx, u.ID, u.Mail)
	if err != nil {
		return AuthResponse{}, err
	}
	return AuthResponse{AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken, User: u}, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (AuthResponse, error) {
	u, err := s.repo.GetUserByEmailAndPassword(ctx, email, password)
	if err != nil {
		return AuthResponse{}, err
	}
	tokens, err := s.issueTokens(ctx, u.ID, u.Mail)
	if err != nil {
		return AuthResponse{}, err
	}
	return AuthResponse{AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken, User: u}, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (AuthTokens, error) {
	claims, err := ParseJWT(s.cfg.Secret, refreshToken, "refresh")
	if err != nil {
		return AuthTokens{}, httpx.Unauthorized("invalid refresh token")
	}
	exists, err := s.repo.HasRefreshToken(ctx, claims.Subject, refreshToken)
	if err != nil {
		return AuthTokens{}, err
	}
	if !exists {
		return AuthTokens{}, httpx.Unauthorized("refresh token not found or expired")
	}
	if err := s.repo.DeleteRefreshToken(ctx, refreshToken); err != nil {
		return AuthTokens{}, err
	}
	return s.issueTokens(ctx, claims.Subject, claims.Email)
}

func (s *Service) Logout(ctx context.Context, userID string) error {
	return s.repo.DeleteRefreshTokensByUser(ctx, userID)
}

func (s *Service) Session(ctx context.Context, userID string) (models.User, error) {
	return s.repo.GetUserByID(ctx, userID)
}

func (s *Service) ChangePassword(ctx context.Context, userID, newPassword string) error {
	if newPassword == "" {
		return httpx.BadRequest("new_password is required")
	}
	return s.repo.UpdatePassword(ctx, userID, newPassword)
}

func (s *Service) VerifyAccessToken(token string) (string, error) {
	claims, err := ParseJWT(s.cfg.Secret, token, "access")
	if err != nil {
		return "", err
	}
	return claims.Subject, nil
}

func (s *Service) issueTokens(ctx context.Context, userID, email string) (AuthTokens, error) {
	access, _, err := GenerateJWT(s.cfg.Secret, userID, email, "access", s.cfg.AccessTTL)
	if err != nil {
		return AuthTokens{}, err
	}
	refresh, refreshExp, err := GenerateJWT(s.cfg.Secret, userID, email, "refresh", s.cfg.RefreshTTL)
	if err != nil {
		return AuthTokens{}, err
	}
	if err := s.repo.SaveRefreshToken(ctx, userID, refresh, refreshExp); err != nil {
		return AuthTokens{}, err
	}
	return AuthTokens{AccessToken: access, RefreshToken: refresh}, nil
}
