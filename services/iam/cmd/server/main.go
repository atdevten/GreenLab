// @title           GreenLab IAM API
// @version         1.0
// @description     Identity and access management service for the GreenLab IoT platform.
// @host            localhost:8001
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
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/greenlab/iam/internal/application"
	"github.com/greenlab/iam/internal/config"
	infraKafka "github.com/greenlab/iam/internal/infrastructure/kafka"
	infraPostgres "github.com/greenlab/iam/internal/infrastructure/postgres"
	infraRedis "github.com/greenlab/iam/internal/infrastructure/redis"
	identityHTTP "github.com/greenlab/iam/internal/transport/http"
	"github.com/greenlab/shared/pkg/logger"
	sharedPostgres "github.com/greenlab/shared/pkg/postgres"
	sharedRedis "github.com/greenlab/shared/pkg/redis"
	"go.uber.org/zap"

	_ "github.com/greenlab/iam/docs"
)

func main() {
	cfg := config.Load()

	logger.Init(logger.Config{Level: cfg.Log.Level, Encoding: cfg.Log.Encoding})
	log := logger.L()

	privateKey, publicKey := loadRSAKeys(log, &cfg.JWT)

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

	producer := infraKafka.NewEventProducer(cfg.Kafka.Brokers)
	defer producer.Close()

	// Auth dependencies
	userRepo := infraPostgres.NewUserRepo(db)
	tokenRepo := infraPostgres.NewTokenRepo(db)
	cache := infraRedis.NewCache(rdb)

	// Tenant dependencies
	orgRepo := infraPostgres.NewOrgRepo(db)
	wsRepo := infraPostgres.NewWorkspaceRepo(db)
	apiKeyRepo := infraPostgres.NewAPIKeyRepo(db)

	authSvc := application.NewAuthService(
		userRepo, tokenRepo, cache, producer,
		orgRepo,
		privateKey, publicKey,
		cfg.JWT.Issuer,
	)

	tenantSvc := application.NewTenantService(orgRepo, wsRepo, apiKeyRepo)

	// Handlers & router
	authHandler := identityHTTP.NewAuthHandler(authSvc)
	tenantHandler := identityHTTP.NewTenantHandler(tenantSvc)
	router := identityHTTP.NewRouter(authHandler, tenantHandler, publicKey, log)

	srv := &http.Server{
		Addr:         ":" + cfg.HTTP.Port,
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	go func() {
		log.Info("identity service starting", zap.String("port", cfg.HTTP.Port))
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
	log.Info("identity service stopped")
}

func loadRSAKeys(log *zap.Logger, cfg *config.JWTConfig) (*rsa.PrivateKey, *rsa.PublicKey) {
	privPEM, err := os.ReadFile(cfg.PrivateKeyPath)
	if err != nil {
		log.Fatal("read private key", zap.Error(err))
	}
	pubPEM, err := os.ReadFile(cfg.PublicKeyPath)
	if err != nil {
		log.Fatal("read public key", zap.Error(err))
	}
	privKey, err := parsePrivateKey(privPEM)
	if err != nil {
		log.Fatal("parse private key", zap.Error(err))
	}
	pubKey, err := parsePublicKey(pubPEM)
	if err != nil {
		log.Fatal("parse public key", zap.Error(err))
	}
	return privKey, pubKey
}

func parsePrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA private key")
	}
	return rsaKey, nil
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
