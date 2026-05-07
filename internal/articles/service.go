package articles

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"

	"isf-backend/internal/cache"
)

type Repository interface {
	Create(ctx context.Context, req CreateArticleRequest) (*Article, error)
	GetBySlug(ctx context.Context, slug string) (*Article, error)
	List(ctx context.Context, includeUnpublished bool) ([]Article, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateArticleRequest) (*Article, error)
	Delete(ctx context.Context, id uuid.UUID) (string, error)
}

type Service struct {
	repo  Repository
	cache cache.Cache
}

func NewService(repo Repository, c cache.Cache) *Service {
	return &Service{repo: repo, cache: c}
}

func articleKey(slug string) string {
	return "article:slug:" + slug
}

const cacheTTL = 5 * time.Minute

func (s *Service) Create(ctx context.Context, req CreateArticleRequest) (*Article, error) {
	if req.SourceLang == "" {
		req.SourceLang = "en"
	}
	return s.repo.Create(ctx, req)
}

func (s *Service) Get(ctx context.Context, slug string) (*Article, error) {
	key := articleKey(slug)

	if data, err := s.cache.Get(ctx, key); err == nil {
		var a Article
		jerr := json.Unmarshal(data, &a)
		if jerr == nil {
			return &a, nil
		}
		log.Printf("cache: bad payload at %s, refreshing: %v", key, jerr)
	} else if !errors.Is(err, cache.ErrMiss) {
		log.Printf("cache: get failed for %s: %v", key, err)
	}
	//cache miss
	a, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if data, jerr := json.Marshal(a); jerr == nil {
		if cerr := s.cache.Set(ctx, key, data, cacheTTL); cerr != nil {
			log.Printf("cache: set failed for %s: %v", key, cerr)
		}
	}

	return a, nil
}

func (s *Service) List(ctx context.Context, includeUnpublished bool) ([]Article, error) {
	return s.repo.List(ctx, includeUnpublished)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateArticleRequest) (*Article, error) {
	a, err := s.repo.Update(ctx, id, req)
	if err != nil {
		return nil, err
	}
	if cerr := s.cache.Del(ctx, articleKey(a.Slug)); cerr != nil {
		log.Printf("cache: del failed for %s: %v", a.Slug, cerr)
	}

	return a, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	slug, err := s.repo.Delete(ctx, id)
	if err != nil {
		return err
	}
	if cerr := s.cache.Del(ctx, articleKey(slug)); cerr != nil {
		log.Printf("cache: del failed for %s: %v", slug, cerr)
	}
	return nil
}
