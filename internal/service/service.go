package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"blog/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

type Claims struct {
	UserID uuid.UUID
	Role   string
}
type TokenManager struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	return &TokenManager{[]byte(secret), ttl}
}
func (m *TokenManager) Issue(user domain.User) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{"sub": user.ID.String(), "role": user.Role, "iat": now.Unix(), "exp": now.Add(m.ttl).Unix(), "iss": "blog-api"}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}
func (m *TokenManager) Parse(raw string) (Claims, error) {
	token, err := jwt.Parse(raw, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, domain.ErrUnauthorized
		}
		return m.secret, nil
	}, jwt.WithIssuer("blog-api"), jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !token.Valid {
		return Claims{}, domain.ErrUnauthorized
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return Claims{}, domain.ErrUnauthorized
	}
	id, err := uuid.Parse(fmt.Sprint(claims["sub"]))
	if err != nil {
		return Claims{}, domain.ErrUnauthorized
	}
	return Claims{UserID: id, Role: fmt.Sprint(claims["role"])}, nil
}

type Auth struct {
	users  domain.UserRepository
	tokens *TokenManager
}

func NewAuth(users domain.UserRepository, tokens *TokenManager) *Auth { return &Auth{users, tokens} }
func (s *Auth) Register(ctx context.Context, email, name, password string) (*domain.User, string, error) {
	email, name = strings.ToLower(strings.TrimSpace(email)), strings.TrimSpace(name)
	if !strings.Contains(email, "@") || len(name) < 2 || len(password) < 8 {
		return nil, "", domain.ErrInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("hash password: %w", err)
	}
	user := &domain.User{ID: uuid.New(), Email: email, DisplayName: name, PasswordHash: string(hash), Role: "author"}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, "", err
	}
	token, err := s.tokens.Issue(*user)
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}
func (s *Auth) Login(ctx context.Context, email, password string) (*domain.User, string, error) {
	user, err := s.users.ByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, "", domain.ErrUnauthorized
		}
		return nil, "", err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return nil, "", domain.ErrUnauthorized
	}
	token, err := s.tokens.Issue(*user)
	return user, token, err
}

type PostInput struct {
	Title, Excerpt, Content, Status string
	CategoryID                      *uuid.UUID
	Tags                            []string
}
type Posts struct{ repo domain.PostRepository }

func NewPosts(repo domain.PostRepository) *Posts { return &Posts{repo} }
func (s *Posts) Create(ctx context.Context, author uuid.UUID, input PostInput) (*domain.Post, error) {
	if err := validatePost(input); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	post := &domain.Post{ID: uuid.New(), AuthorID: author, CategoryID: input.CategoryID, Title: strings.TrimSpace(input.Title), Slug: slug(input.Title), Excerpt: strings.TrimSpace(input.Excerpt), Content: strings.TrimSpace(input.Content), Status: input.Status}
	if post.Status == "published" {
		post.PublishedAt = &now
	}
	if err := s.repo.Create(ctx, post, normalizeTags(input.Tags)); err != nil {
		return nil, err
	}
	return post, nil
}
func (s *Posts) Update(ctx context.Context, id, actor uuid.UUID, admin bool, input PostInput) (*domain.Post, error) {
	post, err := s.repo.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if post.AuthorID != actor && !admin {
		return nil, domain.ErrForbidden
	}
	if err := validatePost(input); err != nil {
		return nil, err
	}
	post.Title, post.Slug, post.Excerpt, post.Content, post.Status, post.CategoryID = strings.TrimSpace(input.Title), slug(input.Title), strings.TrimSpace(input.Excerpt), strings.TrimSpace(input.Content), input.Status, input.CategoryID
	if post.Status == "published" && post.PublishedAt == nil {
		now := time.Now().UTC()
		post.PublishedAt = &now
	}
	if post.Status != "published" {
		post.PublishedAt = nil
	}
	if err := s.repo.Update(ctx, post, normalizeTags(input.Tags)); err != nil {
		return nil, err
	}
	return post, nil
}
func (s *Posts) Delete(ctx context.Context, id, actor uuid.UUID, admin bool) error {
	post, err := s.repo.ByID(ctx, id)
	if err != nil {
		return err
	}
	if post.AuthorID != actor && !admin {
		return domain.ErrForbidden
	}
	return s.repo.Delete(ctx, id)
}
func (s *Posts) BySlug(ctx context.Context, value string, includePrivate bool) (*domain.Post, error) {
	return s.repo.BySlug(ctx, value, includePrivate)
}
func (s *Posts) List(ctx context.Context, page domain.Page, includePrivate bool) ([]domain.Post, error) {
	page = normalizePage(page)
	return s.repo.List(ctx, page, includePrivate)
}

type Comments struct {
	repo  domain.CommentRepository
	posts domain.PostRepository
}

func NewComments(repo domain.CommentRepository, posts domain.PostRepository) *Comments {
	return &Comments{repo, posts}
}
func (s *Comments) Create(ctx context.Context, postID, authorID uuid.UUID, body string) (*domain.Comment, error) {
	body = strings.TrimSpace(body)
	if body == "" || len(body) > 5000 {
		return nil, domain.ErrInvalid
	}
	if _, err := s.posts.ByID(ctx, postID); err != nil {
		return nil, err
	}
	c := &domain.Comment{ID: uuid.New(), PostID: postID, AuthorID: authorID, Body: body, Status: "visible"}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}
func (s *Comments) List(ctx context.Context, postID uuid.UUID, page domain.Page) ([]domain.Comment, error) {
	return s.repo.ListByPost(ctx, postID, normalizePage(page))
}
func (s *Comments) Delete(ctx context.Context, id, actor uuid.UUID, admin bool) error {
	return s.repo.Delete(ctx, id, actor, admin)
}

func validatePost(in PostInput) error {
	if len(strings.TrimSpace(in.Title)) < 3 || strings.TrimSpace(in.Content) == "" {
		return domain.ErrInvalid
	}
	switch in.Status {
	case "draft", "published", "archived":
		return nil
	default:
		return domain.ErrInvalid
	}
}
func normalizePage(p domain.Page) domain.Page {
	if p.Limit <= 0 {
		p.Limit = 20
	}
	if p.Limit > 100 {
		p.Limit = 100
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return p
}
func slug(value string) string {
	value = strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(strings.TrimSpace(value)), "-"), "-")
	if value == "" {
		return uuid.NewString()
	}
	return value
}
func normalizeTags(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" && !seen[strings.ToLower(v)] {
			seen[strings.ToLower(v)] = true
			out = append(out, v)
		}
	}
	return out
}
