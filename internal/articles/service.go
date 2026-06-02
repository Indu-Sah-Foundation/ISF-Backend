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
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"

	"isf-backend/internal/cache"
)


var mdRenderer = goldmark.New(goldmark.WithRendererOptions(html.WithUnsafe()))

// htmlBodyRe — does the body look like HTML so we can skip markdown
// rendering? Also matches a leading HTML comment (e.g. our thumbnail
// marker `<!-- thumbnail: ... -->`) so bodies that start with one
// don't get pushed through goldmark unnecessarily.
var htmlBodyRe = regexp.MustCompile(`(?i)^\s*(<!--|<(p|h[1-6]|div|ul|ol|blockquote|figure|span|strong|em|img|article|section)\b)`)

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

// numberRe — standalone integers / decimals in body text. Wrapped in
// translate="no" so they always render as Western digits. Without this
// Azure transliterates SOME numbers to the target script ("120" →
// "१२०" in Hindi) and leaves OTHERS as Western, producing the visually
// jarring mix Nepali readers see today.
var numberRe = regexp.MustCompile(`(^|[\s>(\[])([0-9]+(?:[.,][0-9]+)*)\b`)

// properNounRe — names, acronyms, programs that should appear
// verbatim in every language. Add to this list as needed.
var properNounRe = regexp.MustCompile(
	`\b(` +
		// Foundation + programs
		`Indu Sah Foundation|ISF SMILE|ISF Robotics|ISFVolunteers|ISF|` +
		// Robotics partner orgs / programs
		`FIRST Lego League|FIRST LEGO League|FIRST Tech Challenge|FLL|FTC|` +
		// Places (towns, districts, schools)
		`Mahottari|Loharpatti|Janakpurdham|Janakpur|Mithila|Madhepura|Rajbiraj|` +
		`Shiva International Secondary School|Angels School|` +
		// Founders + leadership
		`Lal Sah|Dr\. Vijay Sah|Vijay Sah|Shubham Sah|` +
		// Field staff / volunteers / collaborators named in articles
		`Dr\. Sneha Mahato|Sneha Mahato|Anita Kumari|Mahesh|Rupesh|Santosh|Pappu|Nitesh|Nikhil|Neha|` +
		// Advisory board
		`Dr\. Amit Saini|Dr\. Arne Drews|Dr\. Darren Weiss|Dr\. Stephen Forrest|Dr\. Pravin Shah|` +
		// Partner organizations
		`Rotary Club of Waukee|Rotary International|NepalMed|Humble Smile Foundation|Global Oral Health Foundation Society` +
		`)\b`)

// Azure Translator officially documents `class="notranslate"` as the
// reliable signal to leave content alone. The previously-used
// `translate="no"` is the W3C standard but Azure strips the wrapper
// entirely in many cases (leaving the inner text exposed and unable
// to be matched by the restore loop). `notranslate` is honored AND the
// wrapper survives the round-trip.
const (
	imgPlaceholderOpen  = `<span class="notranslate isf-img-tok">`
	imgPlaceholderClose = `</span>`
)

func imgPlaceholder(i int) string {
	return imgPlaceholderOpen + fmt.Sprint(i) + imgPlaceholderClose
}

// wrapNoTranslate wraps a chunk in <span class="notranslate"> — Azure's
// documented marker for content to skip. More reliable than
// translate="no" which Azure inconsistently strips.
func wrapNoTranslate(s string) string {
	return `<span class="notranslate">` + s + `</span>`
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
		return wrapNoTranslate(m)
	})

	// Numbers last — wrap every standalone integer/decimal in body
	// text so Azure can't transliterate it. Keeps digits Western
	// across every target language, matching what modern Nepali /
	// Hindi web copy actually does (mixed-script numerals look broken).
	html = numberRe.ReplaceAllStringFunc(html, func(m string) string {
		sub := numberRe.FindStringSubmatch(m)
		if len(sub) < 3 {
			return m
		}
		return sub[1] + wrapNoTranslate(sub[2])
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

	// FIRST: wrap URLs / emails / hashtags / proper-nouns / numbers
	// in <span class="notranslate"> *while we still have the original
	// HTML* (anchors, paragraph text — but not yet our own placeholder
	// spans). This avoids the bug where numberRe would otherwise match
	// the digit content of our img/comment placeholder spans (e.g.
	// `<span class="notranslate isf-img-tok">0</span>`) and try to
	// double-wrap them.
	htmlBody = protectInlineTokens(htmlBody)

	// THEN: pull HTML comments out — they hide our thumbnail URL and
	// shouldn't be exposed to Azure at all. Restored unchanged on the
	// other side so the thumbnail metadata survives the round-trip.
	comments := make([]string, 0)
	htmlBody = htmlCommentRe.ReplaceAllStringFunc(htmlBody, func(c string) string {
		i := len(comments)
		comments = append(comments, c)
		return `<span class="notranslate isf-cmt-tok">` + fmt.Sprint(i) + `</span>`
	})

	// Pull <img> tags out and replace with placeholders.
	imgs := make([]string, 0)
	htmlBody = imgTagRe.ReplaceAllStringFunc(htmlBody, func(tag string) string {
		i := len(imgs)
		imgs = append(imgs, tag)
		return imgPlaceholder(i)
	})

	// protectInlineTokens already ran above (before placeholder
	// extraction) — no need to re-run here.
	withPlaceholders := htmlBody

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
			// No match by either path. Log loudly so production catches
			// translator regressions instead of silently shipping broken
			// pages. The sanity gate in Get() then refuses to cache.
			log.Printf("articles: WARN unmatched img placeholder %d in translated body (img stayed missing)", i)
		}

		// ----- Restore HTML comments -----
		for i, c := range comments {
			tok := `<span class="notranslate isf-cmt-tok">` + fmt.Sprint(i) + `</span>`
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

		// IMPORTANT: do NOT strip the `<span translate="no">` wrappers
		// around proper nouns / URLs / numbers. Azure's HTML normalizer
		// frequently EATS the whitespace between an `</span>` and the
		// next word in the target script — so stripping the wrapper
		// produces smushed text like "Indu Sah Foundationमानवीय".
		// Leaving the span in place is visually identical (spans render
		// as invisible inline containers) and preserves spacing
		// because Azure has to keep whitespace INSIDE the span content.
		return out
	}

	return withPlaceholders, restore
}

// stripNoTranslateWrappers removes the bare <span class="notranslate">…</span>
// wrappers we added around URLs/emails/etc, leaving the inner text in
// place. It does NOT touch the typed tokens (isf-img-tok / isf-cmt-tok)
// — those are removed during the dedicated img/comment restore loops.
// Also strips legacy `translate="no"` wrappers from any rows still
// holding them, for cleanup convenience.
var noTransWrapperRe = regexp.MustCompile(
	`<span (?:class=["']notranslate["']|translate=["']no["'])(?:\s+(?:class|translate)=["'][^"']*["'])?>([^<]*)</span>`)

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

var (
	glueBeforeSpanRe = regexp.MustCompile(`([\p{L}\p{N}\p{M}])(<span class="notranslate")`)
	glueAfterSpanRe  = regexp.MustCompile(`(</span>)([\p{L}\p{N}\p{M}])`)
)

func fixNoTranslateSpacing(s string) string {
	s = glueBeforeSpanRe.ReplaceAllString(s, "$1 $2")
	s = glueAfterSpanRe.ReplaceAllString(s, "$1 $2")
	return s
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

		// Titles: we DO strip wrappers because titles render inside an
		// <h1> and a stray <span> inside a heading is uglier than the
		// rare whitespace edge case (titles don't usually have mixed
		// scripts back-to-back).
		title = stripNoTranslateWrappers(translated[0])
		body = fixNoTranslateSpacing(restoreImages(translated[1]))

		// ----- Sanity gates: never cache an obviously broken translation.
		// Each failure path serves the translation for THIS request (so
		// the reader isn't blocked) but skips the SaveTranslation call,
		// so the next request retries fresh.
		fail := func(reason string) (*Article, error) {
			log.Printf("articles: WARN %s for %s/%s, NOT caching", reason, slug, lang)
			localized := *original
			localized.Title = title
			localized.BodyMD = body
			return &localized, nil
		}

		if strings.Contains(body, `class="isf-img-tok"`) ||
			strings.Contains(body, `class="isf-cmt-tok"`) {
			return fail("translated body still contains placeholders")
		}
		if title == "" {
			log.Printf("articles: WARN empty title for %s/%s, serving source", slug, lang)
			return original, nil
		}
		// IMAGE-COUNT GATE — the critical one.
		// If Azure strips our placeholder wrappers entirely (it does this
		// sometimes for `<span class="notranslate">` despite the docs),
		// the typed-token check above WON'T trip because there's no class
		// left to find — only the raw digit. So we count actual <img>
		// tags in the original vs the restored body. Mismatch = some
		// images failed to restore, body is broken, do NOT cache.
		srcImgCount := len(imgTagRe.FindAllString(original.BodyMD, -1))
		dstImgCount := len(imgTagRe.FindAllString(body, -1))
		if srcImgCount != dstImgCount {
			return fail(fmt.Sprintf(
				"image count mismatch (source has %d <img>, translation has %d) — Azure likely stripped placeholder wrappers",
				srcImgCount, dstImgCount))
		}
		// Length sanity: catch the case where Azure returns a stub
		// (truncation, silent failure, content-policy block) instead
		// of a real translation. The naive "byte length" check is
		// misleading because byte counts vary 3x across UTF-8 scripts
		// (Devanagari = 3 bytes/char vs Latin = 1). We compare
		// CHARACTER counts instead.
		//
		// Skip entirely for short source bodies — a 50-word post might
		// legitimately translate into 30 words depending on language,
		// and the false-positive cost outweighs the benefit.
		srcChars := utf8.RuneCountInString(original.BodyMD)
		dstChars := utf8.RuneCountInString(body)
		const (
			minSourceChars = 200 // skip check below this size
			minRatio       = 30  // %, drastically permissive
		)
		if srcChars > minSourceChars && dstChars*100 < srcChars*minRatio {
			return fail(fmt.Sprintf(
				"translated body shrank suspiciously (%d→%d chars, %d%% of source — threshold %d%%)",
				srcChars, dstChars, dstChars*100/max(srcChars, 1), minRatio))
		}

		log.Printf("articles: translated %s → %s (title=%dB, body=%dB), saving",
			slug, lang, len(title), len(body))
		// CRITICAL: use a detached context for the save. The request
		// context (`ctx`) is cancelled the moment the reader closes
		// their tab or navigates away. If a cold translation took 10s
		// and the reader bounced at 8s, ctx.Done() fires and
		// SaveTranslation gets cancelled mid-INSERT — the work is
		// thrown away and the NEXT reader pays the same 10s again.
		// This was the leading cause of "translations sometimes don't
		// save". We give the DB write its own 10-second budget,
		// independent of whether the original requester is still
		// listening.
		saveCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		go func(articleID uuid.UUID, lang, title, body string) {
			defer cancel()
			if serr := s.repo.SaveTranslation(saveCtx, articleID, lang, title, body); serr != nil {
				log.Printf("articles: SAVE TRANSLATION FAILED for %s/%s: %v", slug, lang, serr)
				return
			}
			log.Printf("articles: saved translation %s/%s to DB", slug, lang)
		}(original.ID, lang, title, body)
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
