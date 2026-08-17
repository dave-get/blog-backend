package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"blog/internal/domain"
	"github.com/google/uuid"
)

type fakeUsers struct {
	created *domain.User
	found   *domain.User
	err     error
}

func (f *fakeUsers) Create(_ context.Context, u *domain.User) error        { f.created = u; return f.err }
func (f *fakeUsers) ByEmail(context.Context, string) (*domain.User, error) { return f.found, f.err }

func TestRegisterNormalizesUserAndIssuesToken(t *testing.T) {
	repo := &fakeUsers{}
	tokens := NewTokenManager("this-is-a-test-secret-with-32-characters", time.Minute)
	auth := NewAuth(repo, tokens)
	user, raw, err := auth.Register(context.Background(), " Author@Example.COM ", " Author ", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "author@example.com" || user.DisplayName != "Author" {
		t.Fatalf("unexpected normalized user: %+v", user)
	}
	if user.PasswordHash == "password123" {
		t.Fatal("password was not hashed")
	}
	claims, err := tokens.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != user.ID || claims.Role != "author" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}
func TestRegisterRejectsWeakInput(t *testing.T) {
	auth := NewAuth(&fakeUsers{}, NewTokenManager("this-is-a-test-secret-with-32-characters", time.Minute))
	_, _, err := auth.Register(context.Background(), "invalid", "A", "short")
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

type fakePosts struct {
	post    *domain.Post
	created *domain.Post
	tags    []string
	err     error
}

func (f *fakePosts) Create(_ context.Context, p *domain.Post, tags []string) error {
	f.created = p
	f.tags = tags
	return f.err
}
func (f *fakePosts) Update(_ context.Context, p *domain.Post, tags []string) error {
	f.post = p
	f.tags = tags
	return f.err
}
func (f *fakePosts) Delete(context.Context, uuid.UUID) error                    { return f.err }
func (f *fakePosts) ByID(context.Context, uuid.UUID) (*domain.Post, error)      { return f.post, f.err }
func (f *fakePosts) BySlug(context.Context, string, bool) (*domain.Post, error) { return f.post, f.err }
func (f *fakePosts) List(context.Context, domain.Page, bool) ([]domain.Post, error) {
	return nil, f.err
}

func TestCreatePublishedPost(t *testing.T) {
	repo := &fakePosts{}
	posts := NewPosts(repo)
	author := uuid.New()
	post, err := posts.Create(context.Background(), author, PostInput{Title: " Hello, Go! ", Content: "body", Status: "published", Tags: []string{"Go", "go", " API "}})
	if err != nil {
		t.Fatal(err)
	}
	if post.Slug != "hello-go" {
		t.Fatalf("unexpected slug %q", post.Slug)
	}
	if post.PublishedAt == nil {
		t.Fatal("published timestamp is missing")
	}
	if len(repo.tags) != 2 || repo.tags[0] != "Go" || repo.tags[1] != "API" {
		t.Fatalf("unexpected tags: %#v", repo.tags)
	}
}
func TestUpdateRejectsDifferentAuthor(t *testing.T) {
	owner := uuid.New()
	repo := &fakePosts{post: &domain.Post{ID: uuid.New(), AuthorID: owner}}
	_, err := NewPosts(repo).Update(context.Background(), repo.post.ID, uuid.New(), false, PostInput{Title: "Valid", Content: "body", Status: "draft"})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}
func TestAdminCanUpdateDifferentAuthor(t *testing.T) {
	repo := &fakePosts{post: &domain.Post{ID: uuid.New(), AuthorID: uuid.New()}}
	_, err := NewPosts(repo).Update(context.Background(), repo.post.ID, uuid.New(), true, PostInput{Title: "Valid", Content: "body", Status: "draft"})
	if err != nil {
		t.Fatal(err)
	}
}
