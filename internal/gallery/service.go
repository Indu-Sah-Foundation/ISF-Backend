package gallery

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, req CreateItemRequest) (*Item, error)
	List(ctx context.Context, includeUnpublished bool) ([]Item, error)
	AllSrcs(ctx context.Context) ([]string, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateItemRequest) (*Item, error)
	Delete(ctx context.Context, id uuid.UUID) (string, error)

	ListTags(ctx context.Context) ([]Tag, error)
	CreateTag(ctx context.Context, req CreateTagRequest) (*Tag, error)
	DeleteTag(ctx context.Context, id uuid.UUID) error
}

// BlobDeleter is the interface the storage module satisfies — when an
// item is deleted from the gallery, the corresponding blob is removed
// from Azure too so the bytes don't sit around orphaned.
type BlobDeleter interface {
	DeleteByURL(ctx context.Context, url string) error
}

type Service struct {
	repo Repository
	blob BlobDeleter // optional — nil during tests / when storage is disabled
}

func NewService(repo Repository, blob BlobDeleter) *Service {
	return &Service{repo: repo, blob: blob}
}

func (s *Service) Create(ctx context.Context, req CreateItemRequest) (*Item, error) {
	return s.repo.Create(ctx, req)
}

func (s *Service) List(ctx context.Context, includeUnpublished bool) ([]Item, error) {
	return s.repo.List(ctx, includeUnpublished)
}

func (s *Service) AllSrcs(ctx context.Context) ([]string, error) {
	return s.repo.AllSrcs(ctx)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateItemRequest) (*Item, error) {
	return s.repo.Update(ctx, id, req)
}

func (s *Service) ListTags(ctx context.Context) ([]Tag, error) {
	return s.repo.ListTags(ctx)
}

func (s *Service) CreateTag(ctx context.Context, req CreateTagRequest) (*Tag, error) {
	return s.repo.CreateTag(ctx, req)
}

func (s *Service) DeleteTag(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteTag(ctx, id)
}

// Delete removes the row AND deletes the underlying blob (best-effort —
// blob delete failures are logged but don't fail the request).
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	src, err := s.repo.Delete(ctx, id)
	if err != nil {
		return err
	}
	if s.blob != nil && src != "" {
		// Ignore the error — the DB row is gone, so worst case the
		// orphan cleanup job will mop it up later.
		_ = s.blob.DeleteByURL(ctx, src)
	}
	return nil
}
