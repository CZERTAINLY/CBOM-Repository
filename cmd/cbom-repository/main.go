package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/CZERTAINLY/CBOM-Repository/internal/env"
	"github.com/CZERTAINLY/CBOM-Repository/internal/health"
	internalHttp "github.com/CZERTAINLY/CBOM-Repository/internal/http"
	"github.com/CZERTAINLY/CBOM-Repository/internal/log"
	"github.com/CZERTAINLY/CBOM-Repository/internal/service"
	"github.com/CZERTAINLY/CBOM-Repository/internal/store"
)

func main() {

	// get configuration from environment variables
	cfg, err := env.New()
	if err != nil {
		panic(err)
	}
	initializeLogging(cfg.LogLevel)
	slog.Debug("Service configuration read from environment variables.")

	s3Client, s3Uploader, err := store.ConnectS3(context.Background(), cfg.Store)
	if err != nil {
		slog.Error("Connecting to backend store failed.", slog.String("error", err.Error()))
		os.Exit(1)
	}
	slog.Debug("Connected to backend store.")

	store := store.New(cfg.Store, s3Client, s3Uploader)
	svc, err := service.New(store)
	if err != nil {
		slog.Error("Initializing service layer failed.", slog.String("error", err.Error()))
		os.Exit(1)
	}
	slog.Debug("Service layer initialized.")

	// Initialize health service with storage checker
	storageChecker := health.NewStorageChecker(store)
	healthSvc := health.NewService(storageChecker)
	slog.Debug("Health service initialized.")

	srv := internalHttp.New(cfg.Http, svc, healthSvc)
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Http.Port),
		Handler: srv.Handler(),
	}

	slog.Info("Starting http server.", slog.Int("port", cfg.Http.Port))

	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("`ListenAndServer()` failed.", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func initializeLogging(level slog.Level) {
	base := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: false,
		Level:     level,
	})
	ctxHandler := log.New(base)
	slog.SetDefault(slog.New(ctxHandler))
}
