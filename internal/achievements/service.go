package achievements

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, req CreateAchievementRequest) (*Achievement, error)
	List(ctx context.Context, includeUnpublished bool) ([]Achievement, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateAchievementRequest) (*Achievement, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

func (s *Service) Create(ctx context.Context, req CreateAchievementRequest) (*Achievement, error) {
	return s.repo.Create(ctx, req)
}

func (s *Service) List(ctx context.Context, includeUnpublished bool) ([]Achievement, error) {
	return s.repo.List(ctx, includeUnpublished)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateAchievementRequest) (*Achievement, error) {
	return s.repo.Update(ctx, id, req)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
