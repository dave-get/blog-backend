package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"blog/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Users struct{ db *pgxpool.Pool }

func NewUsers(db *pgxpool.Pool) *Users { return &Users{db} }
func (r *Users) Create(ctx context.Context, u *domain.User) error {
	err := r.db.QueryRow(ctx, `INSERT INTO users (id,email,display_name,password_hash,role) VALUES ($1,$2,$3,$4,$5) RETURNING created_at,updated_at`, u.ID, u.Email, u.DisplayName, u.PasswordHash, u.Role).Scan(&u.CreatedAt, &u.UpdatedAt)
	return mapError(err)
}
func (r *Users) ByEmail(ctx context.Context, email string) (*domain.User, error) {
	u := &domain.User{}
	err := r.db.QueryRow(ctx, `SELECT id,email,display_name,password_hash,role,created_at,updated_at FROM users WHERE email=$1`, email).Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	return u, mapError(err)
}

type Posts struct{ db *pgxpool.Pool }

func NewPosts(db *pgxpool.Pool) *Posts { return &Posts{db} }
func (r *Posts) Create(ctx context.Context, p *domain.Post, tags []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `INSERT INTO posts (id,author_id,category_id,title,slug,excerpt,content,status,published_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING created_at,updated_at`, p.ID, p.AuthorID, p.CategoryID, p.Title, p.Slug, p.Excerpt, p.Content, p.Status, p.PublishedAt).Scan(&p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return mapError(err)
	}
	if err = setTags(ctx, tx, p.ID, tags); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *Posts) Update(ctx context.Context, p *domain.Post, tags []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `UPDATE posts SET category_id=$2,title=$3,slug=$4,excerpt=$5,content=$6,status=$7,published_at=$8,updated_at=NOW() WHERE id=$1 RETURNING updated_at`, p.ID, p.CategoryID, p.Title, p.Slug, p.Excerpt, p.Content, p.Status, p.PublishedAt).Scan(&p.UpdatedAt)
	if err != nil {
		return mapError(err)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM post_tags WHERE post_id=$1`, p.ID); err != nil {
		return err
	}
	if err = setTags(ctx, tx, p.ID, tags); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func setTags(ctx context.Context, tx pgx.Tx, postID uuid.UUID, tags []string) error {
	for _, name := range tags {
		slug := strings.Trim(strings.ToLower(strings.ReplaceAll(name, " ", "-")), "-")
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO tags(name,slug) VALUES($1,$2) ON CONFLICT(slug) DO UPDATE SET name=EXCLUDED.name RETURNING id`, name, slug).Scan(&id); err != nil {
			return mapError(err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO post_tags(post_id,tag_id) VALUES($1,$2)`, postID, id); err != nil {
			return err
		}
	}
	return nil
}
func (r *Posts) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM posts WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
func (r *Posts) ByID(ctx context.Context, id uuid.UUID) (*domain.Post, error) {
	return r.one(ctx, `SELECT id,author_id,category_id,title,slug,excerpt,content,status,published_at,created_at,updated_at FROM posts WHERE id=$1`, id)
}
func (r *Posts) BySlug(ctx context.Context, slug string, private bool) (*domain.Post, error) {
	if private {
		return r.one(ctx, `SELECT id,author_id,category_id,title,slug,excerpt,content,status,published_at,created_at,updated_at FROM posts WHERE slug=$1`, slug)
	}
	return r.one(ctx, `SELECT id,author_id,category_id,title,slug,excerpt,content,status,published_at,created_at,updated_at FROM posts WHERE slug=$1 AND status='published'`, slug)
}
func (r *Posts) one(ctx context.Context, query string, arg any) (*domain.Post, error) {
	p := &domain.Post{}
	err := r.db.QueryRow(ctx, query, arg).Scan(&p.ID, &p.AuthorID, &p.CategoryID, &p.Title, &p.Slug, &p.Excerpt, &p.Content, &p.Status, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, mapError(err)
	}
	p.Tags, err = r.tags(ctx, p.ID)
	return p, err
}
func (r *Posts) List(ctx context.Context, page domain.Page, private bool) ([]domain.Post, error) {
	where := "WHERE status='published'"
	if private {
		where = ""
	}
	q := fmt.Sprintf(`SELECT id,author_id,category_id,title,slug,excerpt,content,status,published_at,created_at,updated_at FROM posts %s ORDER BY COALESCE(published_at,created_at) DESC LIMIT $1 OFFSET $2`, where)
	rows, err := r.db.Query(ctx, q, page.Limit, page.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Post{}
	for rows.Next() {
		var p domain.Post
		if err = rows.Scan(&p.ID, &p.AuthorID, &p.CategoryID, &p.Title, &p.Slug, &p.Excerpt, &p.Content, &p.Status, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Tags, err = r.tags(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
func (r *Posts) tags(ctx context.Context, id uuid.UUID) ([]domain.Tag, error) {
	rows, err := r.db.Query(ctx, `SELECT t.id,t.name,t.slug FROM tags t JOIN post_tags pt ON pt.tag_id=t.id WHERE pt.post_id=$1 ORDER BY t.name`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := []domain.Tag{}
	for rows.Next() {
		var t domain.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

type Comments struct{ db *pgxpool.Pool }

func NewComments(db *pgxpool.Pool) *Comments { return &Comments{db} }
func (r *Comments) Create(ctx context.Context, c *domain.Comment) error {
	return mapError(r.db.QueryRow(ctx, `INSERT INTO comments(id,post_id,author_id,body,status) VALUES($1,$2,$3,$4,$5) RETURNING created_at,updated_at`, c.ID, c.PostID, c.AuthorID, c.Body, c.Status).Scan(&c.CreatedAt, &c.UpdatedAt))
}
func (r *Comments) ListByPost(ctx context.Context, postID uuid.UUID, page domain.Page) ([]domain.Comment, error) {
	rows, err := r.db.Query(ctx, `SELECT id,post_id,author_id,body,status,created_at,updated_at FROM comments WHERE post_id=$1 AND status='visible' ORDER BY created_at LIMIT $2 OFFSET $3`, postID, page.Limit, page.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Comment{}
	for rows.Next() {
		var c domain.Comment
		if err := rows.Scan(&c.ID, &c.PostID, &c.AuthorID, &c.Body, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (r *Comments) Delete(ctx context.Context, id, actor uuid.UUID, admin bool) error {
	query := `DELETE FROM comments WHERE id=$1 AND author_id=$2`
	args := []any{id, actor}
	if admin {
		query = `DELETE FROM comments WHERE id=$1`
		args = []any{id}
	}
	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrConflict
	}
	return err
}
