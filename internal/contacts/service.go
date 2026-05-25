package contacts

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, email, message, ip string) (*Contact, error)
	List(ctx context.Context, limit, offset int) ([]Contact, error)
	UnreadCount(ctx context.Context) (int, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (*Contact, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

func (s *Service) Create(ctx context.Context, email, message, ip string) (*Contact, error) {
	return s.repo.Create(ctx, email, message, ip)
}

func (s *Service) List(ctx context.Context, limit, offset int) ([]Contact, error) {
	return s.repo.List(ctx, limit, offset)
}

func (s *Service) UnreadCount(ctx context.Context) (int, error) {
	return s.repo.UnreadCount(ctx)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (*Contact, error) {
	return s.repo.Update(ctx, id, req)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
