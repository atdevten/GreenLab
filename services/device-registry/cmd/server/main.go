// @title           GreenLab Device Registry API
// @version         1.0
// @description     Device, channel, and field management for the GreenLab IoT platform.
// @host            localhost:8002
// @BasePath        /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

package main

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/greenlab/device-registry/internal/application"
	"github.com/greenlab/device-registry/internal/config"
	infraPostgres "github.com/greenlab/device-registry/internal/infrastructure/postgres"
	infraRedis "github.com/greenlab/device-registry/internal/infrastructure/redis"
	registryHTTP "github.com/greenlab/device-registry/internal/transport/http"
	"github.com/greenlab/shared/pkg/logger"
	sharedPostgres "github.com/greenlab/shared/pkg/postgres"
	sharedRedis "github.com/greenlab/shared/pkg/redis"
	"go.uber.org/zap"

	_ "github.com/greenlab/device-registry/docs"
)

func main() {
	cfg := config.Load()

	logger.Init(logger.Config{Level: cfg.Log.Level, Encoding: cfg.Log.Encoding})
	log := logger.L()

	db := sharedPostgres.Connect(log, sharedPostgres.Config{
		DSN:             cfg.Postgres.DSN,
		MaxOpenConns:    cfg.Postgres.MaxOpenConns,
		MaxIdleConns:    cfg.Postgres.MaxIdleConns,
		ConnMaxLifetime: cfg.Postgres.ConnMaxLifetime,
	})
	defer db.Close()
	mustCheckSchema(db.DB, log)

	rdb := sharedRedis.Connect(log, sharedRedis.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer rdb.Close()

	publicKey := loadPublicKey(log, &cfg.JWT)

	// Device dependencies
	deviceRepo := infraPostgres.NewDeviceRepo(db)
	deviceCache := infraRedis.NewDeviceCache(rdb)
	deviceSvc := application.NewDeviceService(deviceRepo, deviceCache, slog.Default())

	// Channel dependencies
	channelRepo := infraPostgres.NewChannelRepo(db)
	channelSvc := application.NewChannelService(channelRepo)

	// Field dependencies
	fieldRepo := infraPostgres.NewFieldRepo(db)
	fieldSvc := application.NewFieldService(fieldRepo)

	// Provision dependencies (atomic device + channel + fields)
	txRunner := infraPostgres.NewTxRunner(db)
	provisionSvc := application.NewProvisionService(txRunner, deviceCache, slog.Default())

	// Internal (machine-to-machine) dependencies
	internalRepo := infraPostgres.NewInternalRepo(db)
	internalSvc := application.NewInternalService(internalRepo)

	// Handlers & router
	deviceHandler := registryHTTP.NewDeviceHandler(deviceSvc)
	channelHandler := registryHTTP.NewChannelHandler(channelSvc)
	fieldHandler := registryHTTP.NewFieldHandler(fieldSvc)
	internalHandler := registryHTTP.NewInternalHandler(internalSvc)
	provisionHandler := registryHTTP.NewProvisionHandler(provisionSvc)
	router := registryHTTP.NewRouter(deviceHandler, channelHandler, fieldHandler, internalHandler, provisionHandler, publicKey)

	srv := &http.Server{
		Addr:         ":" + cfg.HTTP.Port,
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	go func() {
		log.Info("device-registry service starting", zap.String("port", cfg.HTTP.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("shutdown error", zap.Error(err))
	}
	log.Info("device-registry service stopped")
}

func loadPublicKey(log *zap.Logger, cfg *config.JWTConfig) *rsa.PublicKey {
	pubPEM, err := os.ReadFile(cfg.PublicKeyPath)
	if err != nil {
		log.Fatal("read public key", zap.Error(err))
	}
	key, err := parsePublicKey(pubPEM)
	if err != nil {
		log.Fatal("parse public key", zap.Error(err))
	}
	return key
}

func parsePublicKey(pemBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}
	return rsaKey, nil
}

// mustCheckSchema verifies the DB schema is current before serving traffic.
// Exits with a fatal log if schema_migrations is missing or dirty.
func mustCheckSchema(db *sql.DB, log *zap.Logger) {
	var version uint
	var dirty bool
	if err := db.QueryRow(`SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&version, &dirty); err != nil {
		log.Fatal("schema not initialised — run: make migrate-all", zap.Error(err))
	}
	if dirty {
		log.Fatal("migration in dirty state — manually resolve then run: make migrate-all")
	}
}
