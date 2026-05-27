package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	v1 "github.com/lifei6671/papermind/server/api/v1"
	"github.com/lifei6671/papermind/server/bootstrap/devseed"
	"github.com/lifei6671/papermind/server/bootstrap/migration"
	dbdao "github.com/lifei6671/papermind/server/internal/dao/db"
	"github.com/lifei6671/papermind/server/library/config"
	"github.com/lifei6671/papermind/server/library/utils"
)

func main() {
	configureConsoleLogger()

	cfg, err := config.LoadAndValidate(resolveConfigPath())
	if err != nil {
		slog.Error("load config failed", "error", err)
		os.Exit(1)
	}

	gormDB, err := dbdao.OpenWithOptions(cfg.Database, dbdao.Options{MigrationDir: resolveMigrationRoot()})
	if err != nil {
		slog.Error("open database failed", "error", err)
		os.Exit(1)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		slog.Error("get sql database failed", "error", err)
		os.Exit(1)
	}
	defer utils.SafeClose(sqlDB)

	if err := migration.RunForDriver(gormDB, resolveMigrationRoot(), cfg.Database.Driver); err != nil {
		slog.Error("run database migration failed", "error", err)
		os.Exit(1)
	}
	if err := devseed.RunForDriver(gormDB, cfg.Database.Driver, cfg.App.Env); err != nil {
		slog.Error("seed development data failed", "error", err)
		os.Exit(1)
	}
	server := buildHTTPServer(cfg, v1.NewRouter(v1.RouterOptions{
		DB:                   gormDB,
		AuthSessionProvider:  cfg.Auth.Session.Provider,
		AuthSessionSecret:    cfg.Auth.Session.Secret,
		AuthSessionKeyPrefix: cfg.Auth.Session.KeyPrefix,
		AuthSessionRedisAddr: cfg.Auth.Session.Redis.Addr,
		AuthSessionRedisUser: cfg.Auth.Session.Redis.Username,
		AuthSessionRedisPass: cfg.Auth.Session.Redis.Password,
		AuthSessionRedisDB:   cfg.Auth.Session.Redis.DB,
		AuthSessionTTL:       authSessionTTL(cfg),
		ExportDir:            cfg.Storage.ExportDir,
		UploadDir:            filepath.Join(cfg.Storage.ImportDir, "uploads"),
	}))
	defer utils.SafeClose(server)
	slog.Info("papermind http server starting", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("papermind http server stopped", "error", err)
		os.Exit(1)
	}
}

func configureConsoleLogger() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))
}

func buildHTTPServer(cfg *config.Config, handler http.Handler) *http.Server {
	port := 8080
	if cfg != nil && cfg.App.HTTPPort > 0 {
		port = cfg.App.HTTPPort
	}
	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
	}
}

func authSessionTTL(cfg *config.Config) int {
	if cfg != nil && cfg.Auth.Session.TTL > 0 {
		return cfg.Auth.Session.TTL
	}
	if cfg != nil && cfg.Auth.RefreshTokenTTL > 0 {
		return cfg.Auth.RefreshTokenTTL
	}
	return 0
}

func resolveConfigPath() string {
	if path := os.Getenv("PAPERMIND_CONFIG"); path != "" {
		return path
	}
	return firstExistingPath("conf/app.yaml", "server/conf/app.yaml")
}

func resolveMigrationRoot() string {
	return firstExistingPath("data/migrations", "server/data/migrations")
}

func firstExistingPath(paths ...string) string {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return filepath.Clean(paths[0])
}
