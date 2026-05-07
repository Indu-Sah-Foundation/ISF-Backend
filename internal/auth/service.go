package auth

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type Repository interface {
	GetByEmail(ctx context.Context, email string) (*User, error)
	Create(ctx context.Context, email, passwordHash, role string) (*User, error)
}

type Service struct {
	repo   Repository
	secret string
}

func NewService(repo Repository, jwtSecret string) *Service {
	return &Service{repo: repo, secret: jwtSecret}
}

func (s *Service) Login(ctx context.Context, email, password string) (string, *User, error) {
	u, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", nil, ErrInvalidCredentials
		}
		return "", nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", nil, ErrInvalidCredentials
	}
	token, err := IssueToken(s.secret, u)
	if err != nil {
		return "", nil, err
	}
	return token, u, nil
}

func (s *Service) EnsureAdmin(ctx context.Context, email, password string) error {
	existing, err := s.repo.GetByEmail(ctx, email)
	if err == nil && existing != nil {
		return nil
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if _, err := s.repo.Create(ctx, email, string(hash), "admin"); err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	return nil
}
