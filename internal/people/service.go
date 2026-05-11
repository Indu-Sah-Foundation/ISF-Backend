package people

import (
	"context"
	"github.com/google/uuid"
)

type PeopleRepo interface {
	Create(ctx context.Context, name, email string) (*Person, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Person, error)
	List(ctx context.Context, limit, offset int) ([]Person, error)
}

type Service struct {
	repo PeopleRepo
}

func NewService(repo PeopleRepo) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, name, email string) (*Person, error) {
	return s.repo.Create(ctx, name, email)
}

func (s *Service) List(ctx context.Context, limit, offset int) ([]Person, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.List(ctx, limit, offset)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Person, error) {
	return s.repo.GetByID(ctx, id)
}
