// @title           GreenLab Supporting API
// @version         1.0
// @description     Video stream management and audit event service for the GreenLab IoT platform.
// @host            localhost:8007
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
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"github.com/greenlab/supporting/internal/application"
	"github.com/greenlab/supporting/internal/config"
	infraKafka "github.com/greenlab/supporting/internal/infrastructure/kafka"
	infraPostgres "github.com/greenlab/supporting/internal/infrastructure/postgres"
	infraS3 "github.com/greenlab/supporting/internal/infrastructure/s3"
	supportHTTP "github.com/greenlab/supporting/internal/transport/http"
	sharedPostgres "github.com/greenlab/shared/pkg/postgres"

	_ "github.com/greenlab/supporting/docs"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}

	if m := os.Getenv("GIN_MODE"); m != "" {
		gin.SetMode(m)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	zapLevel, err := zap.ParseAtomicLevel(cfg.Log.Level)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: LOG_LEVEL=%q is not valid, using default (info)\n", cfg.Log.Level)
		zapLevel = zap.NewAtomicLevel()
	}
	zapCfg := zap.NewProductionConfig()
	zapCfg.Level = zapLevel
	log := zap.Must(zapCfg.Build())
	defer log.Sync() //nolint:errcheck

	slog.SetDefault(slog.New(zapslog.NewHandler(log.Core(), zapslog.WithCaller(true))))

	db := sharedPostgres.Connect(log, sharedPostgres.Config{
		DSN:             cfg.Postgres.DSN,
		MaxOpenConns:    cfg.Postgres.MaxOpenConns,
		MaxIdleConns:    cfg.Postgres.MaxIdleConns,
		ConnMaxLifetime: cfg.Postgres.ConnMaxLifetime,
	})
	defer db.Close()
	mustCheckSchema(db.DB, log)

	// S3 storage
	storage, err := infraS3.NewStorage(context.Background(), infraS3.Config{
		Region: cfg.S3.Region,
		Bucket: cfg.S3.Bucket,
	})
	if err != nil {
		log.Fatal("init s3 storage", zap.Error(err))
	}

	// Video dependencies
	streamRepo := infraPostgres.NewStreamRepo(db)
	videoSvc := application.NewVideoService(streamRepo, storage, slog.Default())

	// Audit dependencies
	eventRepo := infraPostgres.NewEventRepo(db)
	auditSvc := application.NewAuditService(eventRepo, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start audit Kafka consumer (user.events → audit records)
	auditConsumer := infraKafka.NewAuditConsumer(cfg.Kafka.Brokers, cfg.Kafka.AuditGroupID, []string{"user.events"}, auditSvc, slog.Default())
	defer func() {
		if err := auditConsumer.Close(); err != nil {
			log.Error("failed to close audit consumer", zap.Error(err))
		}
	}()
	go func() {
		if err := auditConsumer.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("audit consumer error", zap.Error(err))
		}
	}()

	publicKey := loadPublicKey(log, cfg.JWT.PublicKeyPath)

	videoHandler := supportHTTP.NewVideoHandler(videoSvc, slog.Default())
	auditHandler := supportHTTP.NewAuditHandler(auditSvc, slog.Default())
	router := supportHTTP.NewRouter(videoHandler, auditHandler, publicKey)

	srv := &http.Server{
		Addr:              ":" + cfg.HTTP.Port,
		Handler:           router,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	go func() {
		log.Info("supporting service starting", zap.String("port", cfg.HTTP.Port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	cancel()

	log.Info("shutting down...")
	shutCtx, shutCancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error("shutdown error", zap.Error(err))
	}
	log.Info("supporting service stopped")
}

func loadPublicKey(log *zap.Logger, path string) *rsa.PublicKey {
	pubPEM, err := os.ReadFile(path)
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
