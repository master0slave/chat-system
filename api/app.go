package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"supportchat/business/usecases"
	"supportchat/handlers"
	"supportchat/pkg/auth"
	"supportchat/pkg/clock"
	"supportchat/pkg/ids"
	"supportchat/realtime"
	"supportchat/repositories"
)

const devJWTSecret = "dev-only-secret-change-me"

type config struct {
	Port           string
	MongoURI       string
	MongoDB        string
	JWTSecret      string
	AllowedOrigins []string
}

func configFromEnv() config {
	return config{
		Port:           env("PORT", "8080"),
		MongoURI:       env("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:        env("MONGO_DB", "supportchat"),
		JWTSecret:      env("JWT_SECRET", devJWTSecret),
		AllowedOrigins: strings.Split(env("WEB_ORIGINS", "http://localhost:3000"), ","),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// newApp wires adapters to use cases. It is the only place that knows every concrete type.
// The returned function disconnects from MongoDB.
func newApp(ctx context.Context, cfg config, logger *slog.Logger) (*echo.Echo, func(), error) {
	client, err := repositories.Connect(ctx, cfg.MongoURI)
	if err != nil {
		return nil, nil, err
	}
	db := client.Database(cfg.MongoDB)
	if err := repositories.EnsureIndexes(ctx, db); err != nil {
		_ = client.Disconnect(ctx)
		return nil, nil, err
	}
	if cfg.JWTSecret == devJWTSecret {
		logger.Warn("JWT_SECRET is not set; using the development secret")
	}

	hub := realtime.NewHub(realtime.DefaultBufferSize)
	cases := usecases.NewCaseService(usecases.CaseDeps{
		Cases:       repositories.NewMongoCaseRepository(db),
		Messages:    repositories.NewMongoMessageRepository(db),
		Broadcaster: hub,
		Clock:       clock.System{},
		IDs:         ids.UUIDv7{},
	})
	authService := usecases.NewAuthService(auth.NewJWT([]byte(cfg.JWTSecret), 12*time.Hour))

	router := handlers.NewRouter(handlers.Config{
		Auth:           authService,
		Cases:          cases,
		Events:         hub,
		AllowedOrigins: cfg.AllowedOrigins,
		Logger:         logger,
	})
	return router, func() { _ = client.Disconnect(context.Background()) }, nil
}
