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

// htmlCommentRe matches an HTML comment. We strip these before
// translation so the thumbnail URL inside `<!-- thumbnail: ... -->`
// never gets mangled, and we splice them back unchanged after.
var htmlCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)

// bareURLRe matches a standalone http(s) URL in body text only.
// Critically the URL must be preceded by a whitespace character, line
// start, or a `>` (end of a tag) — NOT a `"` or `'` (which would mean
// the URL is inside an attribute like href="..." or src="..."). The
// earlier version wrapped attribute URLs in <span translate="no">,
// which split the attribute value with raw tag bytes and produced
// HTML like  href="<span translate="no">https://…</span>"
// — completely corrupted output.
var bareURLRe = regexp.MustCompile(`(^|[\s>])(https?://[^\s<>"']+)`)

// emailRe matches a plain email address — also a common Translator
// corruption target ("smith@indu.org" → "smith @ indu .org").
var emailRe = regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`)

// hashtagRe — Twitter-style hashtags. Translator may try to translate
// the word after #.
var hashtagRe = regexp.MustCompile(`(?:^|\s)(#[A-Za-z0-9_]{2,30})`)

// properNounRe — names, acronyms, programs that should appear
// verbatim in every language. Add to this list as needed.
var properNounRe = regexp.MustCompile(
	`\b(` +
		`Indu Sah Foundation|ISF SMILE|ISF Robotics|ISF|` +
		`FIRST Lego League|FIRST LEGO League|FIRST Tech Challenge|FLL|FTC|` +
		`Mahottari|Loharpatti|Janakpurdham|Janakpur|Mithila|Madhepura|` +
		`Lal Sah|Dr\. Vijay Sah|Vijay Sah|Shubham Sah|` +
		`Rotary Club of Waukee|Rotary International|` +
		`NepalMed|Humble Smile Foundation|Global Oral Health Foundation Society` +
		`)\b`)

const (
	imgPlaceholderOpen  = `<span translate="no" class="isf-img-tok">`
	imgPlaceholderClose = `</span>`
)

func imgPlaceholder(i int) string {
	return imgPlaceholderOpen + fmt.Sprint(i) + imgPlaceholderClose
}

// wrapNoTranslate wraps a chunk in <span translate="no"> if it isn't
// already inside one. Skip when the surrounding context already has a
// translate="no" ancestor to avoid double-wrapping that some translators
// gag on.
func wrapNoTranslate(s string) string {
	return `<span translate="no">` + s + `</span>`
}

// protectInlineTokens wraps URLs, emails, hashtags and the proper-noun
// list in <span translate="no"> so Azure leaves them untouched.
// Runs AFTER markdown→HTML and AFTER <img> placeholder replacement so
// we don't accidentally wrap a URL inside an <img src=> attribute.
//
// Crucially, this function NEVER wraps content that's inside an HTML
// attribute (href="...", src="...", etc.) — doing so would inject raw
// tag bytes into an attribute value and corrupt the document.
func protectInlineTokens(html string) string {
	// URLs: regex captures leading delimiter (space, >, or line start)
	// so we preserve it. The URL itself is wrapped; the delimiter is
	// re-emitted unchanged.
	html = bareURLRe.ReplaceAllStringFunc(html, func(m string) string {
		sub := bareURLRe.FindStringSubmatch(m)
		if len(sub) < 3 {
			return m
		}
		return sub[1] + wrapNoTranslate(sub[2])
	})

	// Emails: tighten with the same "must be preceded by whitespace,
	// `>`, or line start" rule so an email-shaped string inside an
	// attribute doesn't get wrapped.
	html = emailContextualRe.ReplaceAllStringFunc(html, func(m string) string {
		sub := emailContextualRe.FindStringSubmatch(m)
		if len(sub) < 3 {
			return m
		}
		return sub[1] + wrapNoTranslate(sub[2])
	})

	html = hashtagRe.ReplaceAllStringFunc(html, func(m string) string {
		idx := strings.IndexByte(m, '#')
		if idx < 0 {
			return m
		}
		return m[:idx] + wrapNoTranslate(m[idx:])
	})

	// Proper nouns: word-boundary anchored, only matches in text
	// content. Attribute values rarely contain these (they'd be in
	// hrefs/srcs which are URLs, not English phrases) but if one
	// did appear it'd just be wrapped — the wrapper inside an attr
	// would still be invalid HTML, so we double-check by skipping
	// matches that are immediately preceded by `"` or `'`.
	html = properNounRe.ReplaceAllStringFunc(html, func(m string) string {
		// We can't easily peek behind with ReplaceAllStringFunc; rely
		// on the word-boundary that's already in properNounRe to keep
		// us out of URLs (a `/` before "ISF" wouldn't be a word boundary).
		return wrapNoTranslate(m)
	})
	return html
}

// emailContextualRe is emailRe with a leading-context guard (must be
// preceded by start-of-string, whitespace, or `>`).
var emailContextualRe = regexp.MustCompile(
	`(^|[\s>])([A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,})`)

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

	// Pull HTML comments out FIRST — they hide our thumbnail URL and
	// shouldn't be exposed to Azure at all. Restored unchanged on the
	// other side so the thumbnail metadata survives the round-trip.
	comments := make([]string, 0)
	htmlBody = htmlCommentRe.ReplaceAllStringFunc(htmlBody, func(c string) string {
		i := len(comments)
		comments = append(comments, c)
		return `<span translate="no" class="isf-cmt-tok">` + fmt.Sprint(i) + `</span>`
	})

	// Pull <img> tags out and replace with placeholders.
	imgs := make([]string, 0)
	htmlBody = imgTagRe.ReplaceAllStringFunc(htmlBody, func(tag string) string {
		i := len(imgs)
		imgs = append(imgs, tag)
		return imgPlaceholder(i)
	})

	// Now wrap everything else that mustn't be translated — URLs,
	// emails, hashtags, brand/place names. Done AFTER <img> + comment
	// extraction so we don't wrap URLs inside their attributes.
	withPlaceholders := protectInlineTokens(htmlBody)

	restore := func(translated string) string {
		out := translated

		// ----- Restore <img> tags -----
		for i, tag := range imgs {
			tok := imgPlaceholder(i)
			if strings.Contains(out, tok) {
				out = strings.Replace(out, tok, tag, 1)
				continue
			}
			// Tolerance for Azure HTML-mode quirks (attribute reorder,
			// extra whitespace, quote-style changes).
			loose := regexp.MustCompile(
				`<span\b[^>]*class=["']?isf-img-tok["']?[^>]*>` +
					regexp.QuoteMeta(fmt.Sprint(i)) +
					`</span>`)
			if loc := loose.FindStringIndex(out); loc != nil {
				out = out[:loc[0]] + tag + out[loc[1]:]
				continue
			}
			// Legacy: rows translated under the old ¦¦IMG{n}¦¦ scheme.
			legacy := regexp.MustCompile(
				`¦¦\s*(IMG|आईएमजी|आइएमजी|إيمج|آی\s*ام\s*جی|IMAGEN|IMAGEM)\s*` +
					fmt.Sprint(i) +
					`\s*¦¦`)
			if loc := legacy.FindStringIndex(out); loc != nil {
				out = out[:loc[0]] + tag + out[loc[1]:]
				continue
			}
			// We tried — log loudly so production catches translator
			// regressions instead of silently shipping broken pages.
			log.Printf("articles: WARN unmatched img placeholder %d in translated body (img stayed missing)", i)
		}

		// ----- Restore HTML comments -----
		for i, c := range comments {
			tok := `<span translate="no" class="isf-cmt-tok">` + fmt.Sprint(i) + `</span>`
			if strings.Contains(out, tok) {
				out = strings.Replace(out, tok, c, 1)
				continue
			}
			loose := regexp.MustCompile(
				`<span\b[^>]*class=["']?isf-cmt-tok["']?[^>]*>` +
					regexp.QuoteMeta(fmt.Sprint(i)) +
					`</span>`)
			if loc := loose.FindStringIndex(out); loc != nil {
				out = out[:loc[0]] + c + out[loc[1]:]
			}
		}

		// ----- Strip the translate="no" wrappers we added around
		//       URLs / emails / hashtags / proper nouns. Those served
		//       only to protect the inner text during translation — the
		//       reader doesn't need an extra <span>.
		out = stripNoTranslateWrappers(out)

		return out
	}

	return withPlaceholders, restore
}

// stripNoTranslateWrappers removes the bare <span translate="no">…</span>
// wrappers we added around URLs/emails/etc, leaving the inner text in
// place. It does NOT touch the typed tokens (isf-img-tok / isf-cmt-tok)
// — those are removed during the dedicated img/comment restore loops.
var noTransWrapperRe = regexp.MustCompile(
	`<span translate=["']no["'](?:\s+class=["'][^"']*["'])?>([^<]*)</span>`)

func stripNoTranslateWrappers(s string) string {
	return noTransWrapperRe.ReplaceAllStringFunc(s, func(m string) string {
		// Skip our typed tokens — they're handled separately.
		if strings.Contains(m, "isf-img-tok") || strings.Contains(m, "isf-cmt-tok") {
			return m
		}
		sub := noTransWrapperRe.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		return sub[1]
	})
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
	Translate(ctx context.Context, texts []string, sourceLang, targetLang string) ([]string, error)
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
		//   2. Strip HTML comments + replace <img> with inert placeholders
		//   3. Wrap URLs / emails / hashtags / proper-nouns in translate="no"
		//   4. Hand to Azure with explicit `from=<source>` + `textType=html`
		//   5. On the way back: validate placeholders matched, restore
		//      <img> + comments, strip the translate="no" wrappers
		//   6. If validation fails, log and serve source — never cache a
		//      broken translation in the DB
		bodyForTranslate, restoreImages := prepBodyForTranslation(original.BodyMD)

		// Title is plain text — wrap any proper nouns in it too so
		// "ISF SMILE" stays "ISF SMILE" not "ISF SORRIR" etc.
		titleForTranslate := protectInlineTokens(original.Title)

		sourceLang := original.SourceLang
		if sourceLang == "" {
			sourceLang = "en"
		}

		translated, terr := s.translator.Translate(
			ctx, []string{titleForTranslate, bodyForTranslate}, sourceLang, lang)
		if terr != nil {
			// Don't 500 the reader — log + gracefully serve source.
			log.Printf("articles: TRANSLATE FAILED for %s/%s, serving source: %v", slug, lang, terr)
			return original, nil
		}
		if len(translated) != 2 {
			log.Printf("articles: TRANSLATE returned %d items for %s/%s, expected 2; serving source",
				len(translated), slug, lang)
			return original, nil
		}

		rawTitle := stripNoTranslateWrappers(translated[0])
		title = rawTitle
		body = restoreImages(translated[1])

		// Sanity checks — refuse to cache an obviously broken translation.
		if strings.Contains(body, `class="isf-img-tok"`) ||
			strings.Contains(body, "¦¦IMG") ||
			strings.Contains(body, `class="isf-cmt-tok"`) {
			log.Printf("articles: WARN translated body for %s/%s still contains placeholders, NOT caching",
				slug, lang)
			localized := *original
			localized.Title = title
			localized.BodyMD = body
			return &localized, nil
		}
		if title == "" {
			log.Printf("articles: WARN empty title for %s/%s, serving source", slug, lang)
			return original, nil
		}

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
