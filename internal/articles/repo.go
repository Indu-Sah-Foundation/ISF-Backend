package articles

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) Create(ctx context.Context, req CreateArticleRequest) (*Article, error) {
	const q = `
	INSERT INTO articles (slug, title, body_md, source_lang, published_at)
	VALUES ($1, $2, $3, $4,
			CASE WHEN $5::bool THEN NOW() ELSE NULL END)
	RETURNING id, slug, title, body_md, source_lang, published_at, created_at, updated_at
	`
	var a Article
	err := r.pool.QueryRow(ctx, q, req.Slug, req.Title, req.BodyMD, req.SourceLang, req.Publish).Scan(&a.ID, &a.Slug, &a.Title, &a.BodyMD, &a.SourceLang, &a.PublishedAt, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create article: %w", err)
	}
	return &a, nil
}

func (r *Repo) GetBySlug(ctx context.Context, slug string) (*Article, error) {
	const q = `SELECT id, slug, title, body_md, source_lang, published_at, created_at, updated_at FROM articles WHERE slug = $1`
	var a Article
	err := r.pool.QueryRow(ctx, q, slug).Scan(&a.ID, &a.Slug, &a.Title, &a.BodyMD, &a.SourceLang, &a.PublishedAt, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get article: %w", err)
	}
	return &a, nil
}

func (r *Repo) List(ctx context.Context, includeUnpublished bool, limit, offset int) ([]Article, error) {
	const q = `SELECT id, slug, title, body_md, source_lang, published_at, created_at, updated_at
	FROM articles
	WHERE $1 OR published_at IS NOT NULL
	ORDER BY COALESCE(published_at, created_at) DESC
	LIMIT $2 OFFSET $3`
	rows, err := r.pool.Query(ctx, q, includeUnpublished, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list articles: %w", err)
	}
	defer rows.Close()
	articles := make([]Article, 0)
	for rows.Next() {
		var a Article
		if err := rows.Scan(&a.ID, &a.Slug, &a.Title, &a.BodyMD, &a.SourceLang, &a.PublishedAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan article: %w", err)
		}
		articles = append(articles, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate articles: %w", err)
	}
	return articles, nil
}

func (r *Repo) Update(ctx context.Context, id uuid.UUID, req UpdateArticleRequest) (*Article, error) {
	const q = `
	UPDATE articles
	SET
		title        = COALESCE($2, title),
		body_md      = COALESCE($3, body_md),
		published_at = CASE
						   WHEN $4::bool IS NULL THEN published_at
						   WHEN $4::bool = true THEN COALESCE(published_at, NOW())
						   ELSE NULL
					   END,
		updated_at   = NOW()
	WHERE id = $1
	RETURNING id, slug, title, body_md, source_lang, published_at, created_at, updated_at
`
	var a Article
	err := r.pool.QueryRow(ctx, q, id, req.Title, req.BodyMD, req.Publish).Scan(&a.ID, &a.Slug, &a.Title, &a.BodyMD, &a.SourceLang, &a.PublishedAt, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update article: %w", err)
	}
	return &a, nil
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID) (string, error) {
	const q = `DELETE FROM articles WHERE id = $1 RETURNING slug`
	var slug string
	err := r.pool.QueryRow(ctx, q, id).Scan(&slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("delete article: %w", err)
	}
	return slug, nil
}

func (r *Repo) GetTranslation(ctx context.Context, articleID uuid.UUID, lang string) (title, bodyMD string, err error) {
	const q = `SELECT title, body_md FROM article_translations WHERE article_id = $1 AND lang = $2`
	err = r.pool.QueryRow(ctx, q, articleID, lang).Scan(&title, &bodyMD)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrNotFound
		}
		return "", "", fmt.Errorf("get translation: %w", err)

	}
	return title, bodyMD, nil
}

func (r *Repo) SaveTranslation(ctx context.Context, articleID uuid.UUID, lang, title, bodyMD string) error {
	const q = `
    INSERT INTO article_translations (article_id, lang, title, body_md, translated_at)
    VALUES ($1, $2, $3, $4, NOW())
    ON CONFLICT (article_id, lang) DO UPDATE
        SET title = EXCLUDED.title,
            body_md = EXCLUDED.body_md,
            translated_at = NOW()
    `
	if _, err := r.pool.Exec(ctx, q, articleID, lang, title, bodyMD); err != nil {
		return fmt.Errorf("save translation: %w", err)
	}
	return nil
}

func (r *Repo) DeleteTranslation(ctx context.Context, articleID uuid.UUID) error {
	const q = `DELETE FROM article_translations WHERE article_id = $1`
	if _, err := r.pool.Exec(ctx, q, articleID); err != nil {
		return fmt.Errorf("delete translations: %w", err)
	}
	return nil
}
