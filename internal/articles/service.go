package articles

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"isf-backend/internal/cache"
)

type Repository interface {
	Create(ctx context.Context, req CreateArticleRequest) (*Article, error)
	GetBySlug(ctx context.Context, slug string) (*Article, error)
	List(ctx context.Context, includeUnpublished bool, limit, offset int) ([]Article, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateArticleRequest) (*Article, error)
	Delete(ctx context.Context, id uuid.UUID) (string, error)

	GetTranslation(ctx context.Context, articleID uuid.UUID, lang string) (string, string, error)
	SaveTranslation(ctx context.Context, articleID uuid.UUID, lang, title, bodyMD string) error
	DeleteTranslation(ctx context.Context, articleID uuid.UUID) error
}

type Service struct {
	repo       Repository
	cache      cache.Cache
	translator Translator
}

type Translator interface {
	Translate(ctx context.Context, texts []string, targetLang string) ([]string, error)
}

func NewService(repo Repository, c cache.Cache, t Translator) *Service {
	return &Service{repo: repo, cache: c, translator: t}
}

func articleKey(slug, lang string) string {
	if lang == "" {
		return "article:slug:" + slug
	}
	return "article:slug:" + slug + ":lang:" + lang
}

const cacheTTL = 5 * time.Minute

func (s *Service) Create(ctx context.Context, req CreateArticleRequest) (*Article, error) {
	if req.SourceLang == "" {
		req.SourceLang = "en"
	}
	return s.repo.Create(ctx, req)
}

func (s *Service) Get(ctx context.Context, slug, lang string) (*Article, error) {
	key := articleKey(slug, lang)

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

	original, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	if lang == "" || lang == original.SourceLang {
		s.tryCache(ctx, articleKey(slug, ""), original)
		return original, nil
	}

	title, body, terr := s.repo.GetTranslation(ctx, original.ID, lang)
	if terr != nil && !errors.Is(terr, ErrNotFound) {
		return nil, terr
	}

	if errors.Is(terr, ErrNotFound) {
		translated, terr := s.translator.Translate(ctx, []string{original.Title, original.BodyMD}, lang)
		if terr != nil {
			return nil, fmt.Errorf("translate: %w", terr)
		}
		title, body = translated[0], translated[1]
		if serr := s.repo.SaveTranslation(ctx, original.ID, lang, title, body); serr != nil {
			log.Printf("save translation failed for %s/%s: %v", slug, lang, serr)

		}
	}

	localized := *original
	localized.Title = title
	localized.BodyMD = body

	s.tryCache(ctx, key, &localized)
	return &localized, nil
}

func (s *Service) List(ctx context.Context, includeUnpublished bool, limit, offset int) ([]Article, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.List(ctx, includeUnpublished, limit, offset)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateArticleRequest) (*Article, error) {
	a, err := s.repo.Update(ctx, id, req)
	if err != nil {
		return nil, err
	}
	if derr := s.repo.DeleteTranslation(ctx, a.ID); derr != nil {
		log.Printf("delete translations failed for %s: %v", a.ID, derr)
	}
	if cerr := s.cache.Del(ctx, articleKey(a.Slug, "")); cerr != nil {
		log.Printf("cache: del failed for %s: %v", a.Slug, cerr)
	}

	return a, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	slug, err := s.repo.Delete(ctx, id)
	if err != nil {
		return err
	}
	if cerr := s.cache.Del(ctx, articleKey(slug, "")); cerr != nil {
		log.Printf("cache: del failed for %s: %v", slug, cerr)
	}
	return nil
}

func (s *Service) tryCache(ctx context.Context, key string, a *Article) {
	data, err := json.Marshal(a)
	if err != nil {
		log.Printf("marshal failed to %s: %v", key, err)
		return
	}
	if err := s.cache.Set(ctx, key, data, cacheTTL); err != nil {
		log.Printf("cache set failed for %s: %v", key, err)
	}
}
