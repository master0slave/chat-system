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
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := configFromEnv()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	router, closeDB, err := newApp(startCtx, cfg, logger)
	cancel()
	if err != nil {
		logger.Error("startup failed", "err", err)
		os.Exit(1)
	}
	defer closeDB()

	go func() {
		logger.Info("api listening", "port", cfg.Port)
		if err := router.Start(":" + cfg.Port); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = router.Shutdown(shutdownCtx)
}
