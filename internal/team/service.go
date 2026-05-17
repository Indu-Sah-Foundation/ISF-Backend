package team

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, req CreateMemberRequest) (*Member, error)
	List(ctx context.Context, includeUnpublished bool, kind string) ([]Member, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateMemberRequest) (*Member, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

func (s *Service) Create(ctx context.Context, req CreateMemberRequest) (*Member, error) {
	return s.repo.Create(ctx, req)
}

func (s *Service) List(ctx context.Context, includeUnpublished bool, kind string) ([]Member, error) {
	return s.repo.List(ctx, includeUnpublished, kind)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateMemberRequest) (*Member, error) {
	return s.repo.Update(ctx, id, req)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
