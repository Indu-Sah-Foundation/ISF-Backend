package articles

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"

	"isf-backend/internal/cache"
)


var mdRenderer = goldmark.New(goldmark.WithRendererOptions(html.WithUnsafe()))

var htmlBodyRe = regexp.MustCompile(`(?i)^\s*<(p|h[1-6]|div|ul|ol|blockquote|figure|span|strong|em|img|article|section)\b`)

// imgTagRe matches a full <img …> tag (self-closing or not).
var imgTagRe = regexp.MustCompile(`(?i)<img\b[^>]*>`)

// preTranslateMarker is a token Azure won't touch (no letters / no
// punctuation it would translate). We stuff one of these in place of
// every <img> tag, send the rest to the translator, then put each img
// back exactly. This is bulletproof — images survive intact for every
// language and every Azure quirk.
const imgPlaceholderPrefix = "¦¦IMG"
const imgPlaceholderSuffix = "¦¦"

// prepBodyForTranslation produces (textToTranslate, restoreFn). Pipeline:
//
//  1. If the body looks like markdown, render it to HTML via goldmark so
//     `#`, `*`, `![]` etc. become real tags Azure won't translate.
//  2. Replace every <img> with an inert placeholder token.
//  3. Hand the rest to Azure with textType=html.
//  4. Caller runs restoreFn on the translated string to splice images
//     back in their original order.
func prepBodyForTranslation(body string) (string, func(string) string) {
	htmlBody := body
	if !htmlBodyRe.MatchString(body) {
		var buf bytes.Buffer
		if err := mdRenderer.Convert([]byte(body), &buf); err == nil {
			htmlBody = buf.String()
		} else {
			log.Printf("articles: markdown render failed, using raw body: %v", err)
		}
	}

	// Pull <img> tags out and replace with placeholders.
	imgs := make([]string, 0)
	withPlaceholders := imgTagRe.ReplaceAllStringFunc(htmlBody, func(tag string) string {
		i := len(imgs)
		imgs = append(imgs, tag)
		return imgPlaceholderPrefix + fmt.Sprint(i) + imgPlaceholderSuffix
	})

	restore := func(translated string) string {
		out := translated
		for i, tag := range imgs {
			token := imgPlaceholderPrefix + fmt.Sprint(i) + imgPlaceholderSuffix
			out = strings.Replace(out, token, tag, 1)
			// Azure sometimes capitalizes or spaces the token — try a
			// loose match as a fallback.
			altToken := strings.ReplaceAll(token, " ", "")
			if out == translated && altToken != token {
				out = strings.Replace(out, altToken, tag, 1)
			}
		}
		return out
	}

	return withPlaceholders, restore
}

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
		// Pipeline:
		//   1. Render markdown→HTML if needed
		//   2. Replace <img> tags with inert placeholders
		//   3. Translate
		//   4. Splice the original <img> tags back
		bodyForTranslate, restoreImages := prepBodyForTranslation(original.BodyMD)
		translated, terr := s.translator.Translate(ctx, []string{original.Title, bodyForTranslate}, lang)
		if terr != nil {
			// Don't 500 the reader — log the underlying Azure error and
			// gracefully serve the source-language article instead. Lang
			// dropdowns flip the user back to that view; better than a
			// broken page. We also DON'T cache this in
			// article_translations so the next request will retry.
			log.Printf("articles: TRANSLATE FAILED for %s/%s, serving source: %v", slug, lang, terr)
			return original, nil
		}
		title = translated[0]
		body = restoreImages(translated[1])

		log.Printf("articles: translated %s → %s (title=%dB, body=%dB), saving",
			slug, lang, len(title), len(body))
		if serr := s.repo.SaveTranslation(ctx, original.ID, lang, title, body); serr != nil {
			// Don't return — translation is in hand, still serve the
			// reader. But log loudly so we notice persistent DB issues.
			log.Printf("articles: SAVE TRANSLATION FAILED for %s/%s: %v", slug, lang, serr)
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
