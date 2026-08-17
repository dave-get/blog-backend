package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"blog/internal/domain"
	"blog/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

type API struct {
	auth     *service.Auth
	posts    *service.Posts
	comments *service.Comments
	tokens   *service.TokenManager
	health   func(context.Context) error
	log      *slog.Logger
}

func New(auth *service.Auth, posts *service.Posts, comments *service.Comments, tokens *service.TokenManager, health func(context.Context) error, log *slog.Logger) http.Handler {
	a := &API{auth, posts, comments, tokens, health, log}
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer, middleware.Timeout(15*time.Second), a.logging)
	r.Get("/healthz", a.healthz)
	r.Route("/v1", func(r chi.Router) {
		r.Post("/auth/register", a.register)
		r.Post("/auth/login", a.login)
		r.Get("/posts", a.listPosts)
		r.Get("/posts/{slug}", a.getPost)
		r.Get("/posts/{postID}/comments", a.listComments)
		r.Group(func(r chi.Router) {
			r.Use(a.authenticate)
			r.Post("/posts", a.createPost)
			r.Put("/posts/{postID}", a.updatePost)
			r.Delete("/posts/{postID}", a.deletePost)
			r.Post("/posts/{postID}/comments", a.createComment)
			r.Delete("/comments/{commentID}", a.deleteComment)
		})
	})
	return r
}

type actorKey struct{}

func (a *API) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value := r.Header.Get("Authorization")
		if !strings.HasPrefix(value, "Bearer ") {
			writeError(w, domain.ErrUnauthorized)
			return
		}
		claims, err := a.tokens.Parse(strings.TrimSpace(strings.TrimPrefix(value, "Bearer ")))
		if err != nil {
			writeError(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorKey{}, claims)))
	})
}
func actor(r *http.Request) service.Claims { return r.Context().Value(actorKey{}).(service.Claims) }
func (a *API) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		a.log.Info("http request", "method", r.Method, "path", r.URL.Path, "status", ww.Status(), "bytes", ww.BytesWritten(), "duration_ms", time.Since(start).Milliseconds(), "request_id", middleware.GetReqID(r.Context()))
	})
}

type authRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var in authRequest
	if !decode(w, r, &in) {
		return
	}
	u, t, err := a.auth.Register(r.Context(), in.Email, in.DisplayName, in.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": u, "access_token": t})
}
func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var in authRequest
	if !decode(w, r, &in) {
		return
	}
	u, t, err := a.auth.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": u, "access_token": t})
}

type postRequest struct {
	Title      string     `json:"title"`
	Excerpt    string     `json:"excerpt"`
	Content    string     `json:"content"`
	Status     string     `json:"status"`
	CategoryID *uuid.UUID `json:"category_id"`
	Tags       []string   `json:"tags"`
}

func (i postRequest) input() service.PostInput {
	return service.PostInput{Title: i.Title, Excerpt: i.Excerpt, Content: i.Content, Status: i.Status, CategoryID: i.CategoryID, Tags: i.Tags}
}
func (a *API) listPosts(w http.ResponseWriter, r *http.Request) {
	posts, err := a.posts.List(r.Context(), page(r), false)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": posts})
}
func (a *API) getPost(w http.ResponseWriter, r *http.Request) {
	post, err := a.posts.BySlug(r.Context(), chi.URLParam(r, "slug"), false)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, post)
}
func (a *API) createPost(w http.ResponseWriter, r *http.Request) {
	var in postRequest
	if !decode(w, r, &in) {
		return
	}
	post, err := a.posts.Create(r.Context(), actor(r).UserID, in.input())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, post)
}
func (a *API) updatePost(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "postID")
	if !ok {
		return
	}
	var in postRequest
	if !decode(w, r, &in) {
		return
	}
	c := actor(r)
	post, err := a.posts.Update(r.Context(), id, c.UserID, c.Role == "admin", in.input())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, post)
}
func (a *API) deletePost(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "postID")
	if !ok {
		return
	}
	c := actor(r)
	if err := a.posts.Delete(r.Context(), id, c.UserID, c.Role == "admin"); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type commentRequest struct {
	Body string `json:"body"`
}

func (a *API) listComments(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "postID")
	if !ok {
		return
	}
	items, err := a.comments.List(r.Context(), id, page(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (a *API) createComment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "postID")
	if !ok {
		return
	}
	var in commentRequest
	if !decode(w, r, &in) {
		return
	}
	item, err := a.comments.Create(r.Context(), id, actor(r).UserID, in.Body)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (a *API) deleteComment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "commentID")
	if !ok {
		return
	}
	c := actor(r)
	if err := a.comments.Delete(r.Context(), id, c.UserID, c.Role == "admin"); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) healthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.health(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, domain.ErrInvalid)
		return false
	}
	return true
}
func pathID(w http.ResponseWriter, r *http.Request, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, key))
	if err != nil {
		writeError(w, domain.ErrInvalid)
		return uuid.Nil, false
	}
	return id, true
}
func page(r *http.Request) domain.Page {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	return domain.Page{Limit: limit, Offset: offset}
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "internal_error"
	switch {
	case errors.Is(err, domain.ErrInvalid):
		status, code = http.StatusBadRequest, "invalid_input"
	case errors.Is(err, domain.ErrUnauthorized):
		status, code = http.StatusUnauthorized, "unauthorized"
	case errors.Is(err, domain.ErrForbidden):
		status, code = http.StatusForbidden, "forbidden"
	case errors.Is(err, domain.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, domain.ErrConflict):
		status, code = http.StatusConflict, "conflict"
	}
	if status == http.StatusInternalServerError {
		slog.Error("internal request error", "error", err)
	}
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": code}})
}
