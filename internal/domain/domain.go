package domain

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"time"
)

var (
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("resource already exists")
	ErrUnauthorized = errors.New("invalid credentials")
	ErrForbidden    = errors.New("operation not permitted")
	ErrInvalid      = errors.New("invalid input")
)

type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"display_name"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type Category struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
}
type Tag struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Slug string    `json:"slug"`
}
type Post struct {
	ID          uuid.UUID  `json:"id"`
	AuthorID    uuid.UUID  `json:"author_id"`
	CategoryID  *uuid.UUID `json:"category_id,omitempty"`
	Title       string     `json:"title"`
	Slug        string     `json:"slug"`
	Excerpt     string     `json:"excerpt"`
	Content     string     `json:"content"`
	Status      string     `json:"status"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Tags        []Tag      `json:"tags"`
}
type Comment struct {
	ID        uuid.UUID `json:"id"`
	PostID    uuid.UUID `json:"post_id"`
	AuthorID  uuid.UUID `json:"author_id"`
	Body      string    `json:"body"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type Page struct {
	Limit  int
	Offset int
}

type UserRepository interface {
	Create(context.Context, *User) error
	ByEmail(context.Context, string) (*User, error)
}
type PostRepository interface {
	Create(context.Context, *Post, []string) error
	Update(context.Context, *Post, []string) error
	Delete(context.Context, uuid.UUID) error
	ByID(context.Context, uuid.UUID) (*Post, error)
	BySlug(context.Context, string, bool) (*Post, error)
	List(context.Context, Page, bool) ([]Post, error)
}
type CommentRepository interface {
	Create(context.Context, *Comment) error
	ListByPost(context.Context, uuid.UUID, Page) ([]Comment, error)
	Delete(context.Context, uuid.UUID, uuid.UUID, bool) error
}
