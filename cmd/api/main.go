package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"blog/internal/config"
	"blog/internal/platform/database"
	postgresrepo "blog/internal/repository/postgres"
	"blog/internal/service"
	"blog/internal/transport/httpapi"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	cfg, err := config.Load()
	if err != nil {
		log.Error("load configuration", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	db, err := database.Open(ctx, cfg.DatabaseURL, cfg.DBMinConnections, cfg.DBMaxConnections)
	if err != nil {
		log.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	users := postgresrepo.NewUsers(db)
	posts := postgresrepo.NewPosts(db)
	comments := postgresrepo.NewComments(db)
	tokens := service.NewTokenManager(cfg.JWTSecret, cfg.AccessTokenTTL)
	handler := httpapi.New(service.NewAuth(users, tokens), service.NewPosts(posts), service.NewComments(comments, posts), tokens, db.Ping, log)
	server := &http.Server{Addr: cfg.HTTPAddress, Handler: handler, ReadHeaderTimeout: 5e9, ReadTimeout: 15e9, WriteTimeout: 30e9, IdleTimeout: 60e9}
	go func() {
		log.Info("server started", "address", cfg.HTTPAddress, "environment", cfg.Environment)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server failed", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("server shutdown", "error", err)
	}
}
