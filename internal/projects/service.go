package projects

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, req CreateProjectRequest) (*Project, error)
	Get(ctx context.Context, id uuid.UUID) (*Project, error)
	GetBySlug(ctx context.Context, slug string) (*Project, error)
	List(ctx context.Context, includeUnpublished bool, kind string) ([]Project, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateProjectRequest) (*Project, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

func (s *Service) Create(ctx context.Context, req CreateProjectRequest) (*Project, error) {
	return s.repo.Create(ctx, req)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Project, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) GetBySlug(ctx context.Context, slug string) (*Project, error) {
	return s.repo.GetBySlug(ctx, slug)
}

func (s *Service) List(ctx context.Context, includeUnpublished bool, kind string) ([]Project, error) {
	return s.repo.List(ctx, includeUnpublished, kind)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateProjectRequest) (*Project, error) {
	return s.repo.Update(ctx, id, req)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
