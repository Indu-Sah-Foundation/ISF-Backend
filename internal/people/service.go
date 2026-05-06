package people

import (
	"context"
	"github.com/google/uuid"
)

type PeopleRepo interface {
	Create(ctx context.Context, name, email string) (*Person, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Person, error)
	List(ctx context.Context) ([]Person, error)
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

func (s *Service) List(ctx context.Context) ([]Person, error) {
	return s.repo.List(ctx)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Person, error) {
	return s.repo.GetByID(ctx, id)
}
