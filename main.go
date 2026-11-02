package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chayupon/backend-challenge/internal/auth"
	"github.com/chayupon/backend-challenge/internal/config"
	"github.com/chayupon/backend-challenge/internal/db"
	"github.com/chayupon/backend-challenge/internal/handler"
	"github.com/chayupon/backend-challenge/internal/middleware"
	"github.com/chayupon/backend-challenge/internal/repository"
	"github.com/chayupon/backend-challenge/internal/service"
	"github.com/chayupon/backend-challenge/internal/worker"
	"github.com/labstack/echo/v5"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("application stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	client, err := db.Connect(connectCtx, cfg.MongoURI)
	cancel()
	if err != nil {
		return err
	}
	defer func() {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer disconnectCancel()
		if err := client.Disconnect(disconnectCtx); err != nil {
			logger.Error("disconnect MongoDB", "error", err)
		}
	}()
	repo := repository.NewUser(client.Database(cfg.MongoDatabase))
	indexCtx, indexCancel := context.WithTimeout(ctx, 10*time.Second)
	err = repo.EnsureIndexes(indexCtx)
	indexCancel()
	if err != nil {
		return err
	}
	tokens, err := auth.NewTokens(cfg.JWTSecret, cfg.JWTTTL)
	if err != nil {
		return err
	}
	users, err := service.NewUser(repo, tokens)
	if err != nil {
		return err
	}
	userHandler := handler.NewUser(users)
	e := echo.New()
	e.HTTPErrorHandler = handler.HTTPErrorHandler(logger)
	e.Use(middleware.Logging(logger), middleware.Timeout(5*time.Second))
	registerRoutes(e, userHandler, tokens)
	server := &http.Server{Addr: cfg.HTTPAddress, Handler: e,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	workerCtx, cancelWorker := context.WithCancel(ctx)
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); worker.LogUserCount(workerCtx, repo, logger) }()
	defer func() { cancelWorker(); <-workerDone }()
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	logger.Info("server started", "address", cfg.HTTPAddress)
	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return err
		}
		if err := <-serverErrors; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

func registerRoutes(e *echo.Echo, users *handler.User, tokens *auth.Tokens) {
	e.POST("/auth/register", users.Create)
	e.POST("/auth/login", users.Login)
	protected := e.Group("/users", middleware.Authenticate(tokens))
	protected.POST("", users.Create)
	protected.POST("/get", users.Get)
	protected.GET("", users.List)
	protected.PATCH("", users.Update)
	protected.DELETE("", users.Delete)
}
