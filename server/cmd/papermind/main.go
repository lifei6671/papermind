package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	apirouter "github.com/lifei6671/papermind/server/api/router"
	v1 "github.com/lifei6671/papermind/server/api/v1"
	"github.com/lifei6671/papermind/server/bootstrap/adminseed"
	"github.com/lifei6671/papermind/server/bootstrap/devseed"
	"github.com/lifei6671/papermind/server/bootstrap/migration"
	dbdao "github.com/lifei6671/papermind/server/internal/dao/db"
	"github.com/lifei6671/papermind/server/library/config"
	"github.com/lifei6671/papermind/server/library/utils"
	"gorm.io/gorm"
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
	if err := adminseed.Run(context.Background(), gormDB); err != nil {
		slog.Error("seed default platform admin failed", "error", err)
		os.Exit(1)
	}
	if err := devseed.RunForDriver(gormDB, cfg.Database.Driver, cfg.App.Env); err != nil {
		slog.Error("seed development data failed", "error", err)
		os.Exit(1)
	}
	routerOptions := routerOptionsFromConfig(cfg, gormDB)
	server := buildHTTPServer(cfg, apirouter.New(routerOptions, v1.RegisterRoutes, v1.AuthMiddlewares(routerOptions)...))
	defer utils.SafeClose(server)
	slog.Info("papermind http server starting", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("papermind http server stopped", "error", err)
		os.Exit(1)
	}
}

func routerOptionsFromConfig(cfg *config.Config, gormDB *gorm.DB) apirouter.Options {
	options := apirouter.Options{
		DB:             gormDB,
		AuthSessionTTL: authSessionTTL(cfg),
	}
	if cfg == nil {
		return options
	}
	options.AuthSessionProvider = cfg.Auth.Session.Provider
	options.AuthSessionSecret = cfg.Auth.Session.Secret
	options.AuthSessionKeyPrefix = cfg.Auth.Session.KeyPrefix
	options.AuthSessionRedisAddr = cfg.Auth.Session.Redis.Addr
	options.AuthSessionRedisUser = cfg.Auth.Session.Redis.Username
	options.AuthSessionRedisPass = cfg.Auth.Session.Redis.Password
	options.AuthSessionRedisDB = cfg.Auth.Session.Redis.DB
	options.ExportDir = cfg.Storage.ExportDir
	options.UploadDir = filepath.Join(cfg.Storage.ImportDir, "uploads")
	options.AllowRegisterDefault = cfg.Security.AllowRegisterDefault
	options.PasswordMinLength = cfg.Security.PasswordMinLength
	return options
}

func configureConsoleLogger() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))
}

func buildHTTPServer(cfg *config.Config, handler http.Handler) *http.Server {
	port := 9080
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
	return firstExistingPath("conf/app.yaml", "server/conf/app.yaml", "conf/app.example.yaml", "server/conf/app.example.yaml")
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
