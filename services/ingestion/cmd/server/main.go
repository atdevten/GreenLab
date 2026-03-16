// @title           GreenLab Ingestion API
// @version         1.0
// @description     Telemetry ingestion service for the GreenLab IoT platform.
// @host            localhost:8003
// @BasePath        /
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
// @description Device API key for telemetry ingestion.

package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"github.com/greenlab/ingestion/internal/application"
	"github.com/greenlab/ingestion/internal/config"
	infraAPIKey "github.com/greenlab/ingestion/internal/infrastructure/apikey"
	infraKafka "github.com/greenlab/ingestion/internal/infrastructure/kafka"
	infraPostgres "github.com/greenlab/ingestion/internal/infrastructure/postgres"
	infraRedis "github.com/greenlab/ingestion/internal/infrastructure/redis"
	ingestionHTTP "github.com/greenlab/ingestion/internal/transport/http"
	sharedPostgres "github.com/greenlab/shared/pkg/postgres"
	sharedRedis "github.com/greenlab/shared/pkg/redis"

	_ "github.com/greenlab/ingestion/docs"
)

func main() {
	cfg := config.Load()

	// Build the primary structured logger. Both the zap-based shared packages and
	// the slog-based application layer write through the same zap JSON encoder,
	// producing a single consistent log schema on stdout.
	zapLevel, err := zap.ParseAtomicLevel(cfg.Log.Level)
	if err != nil {
		zapLevel = zap.NewAtomicLevel() // default to info on unrecognised level
	}
	zapCfg := zap.NewProductionConfig()
	zapCfg.Level = zapLevel
	log := zap.Must(zapCfg.Build())
	defer log.Sync() //nolint:errcheck

	// Bridge slog through the same zap core so all log output is uniform.
	slog.SetDefault(slog.New(zapslog.NewHandler(log.Core(), zapslog.WithCaller(true))))

	db := sharedPostgres.Connect(log, sharedPostgres.Config{
		DSN:             cfg.Postgres.DSN,
		MaxOpenConns:    cfg.Postgres.MaxOpenConns,
		MaxIdleConns:    cfg.Postgres.MaxIdleConns,
		ConnMaxLifetime: cfg.Postgres.ConnMaxLifetime,
	})
	defer db.Close()

	rdb := sharedRedis.Connect(log, sharedRedis.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer rdb.Close()

	producer := infraKafka.NewReadingProducer(cfg.Kafka.Brokers)
	defer func() {
		if err := producer.Close(); err != nil {
			log.Error("failed to close kafka producer", zap.Error(err))
		}
	}()

	apiKeyCache := infraRedis.NewAPIKeyCache(rdb)
	deviceStore := infraPostgres.NewDeviceStore(db.DB)
	apiKeyValidator := infraAPIKey.NewValidator(apiKeyCache, deviceStore, slog.Default())

	svc := application.NewIngestService(producer, slog.Default(), cfg.Ingest.MaxReadingAge)
	handler := ingestionHTTP.NewHandler(svc, slog.Default())
	router := ingestionHTTP.NewRouter(handler, apiKeyValidator.Validate, slog.Default(), rdb)

	srv := &http.Server{
		Addr:         ":" + cfg.HTTP.Port,
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	go func() {
		log.Info("ingestion service starting", zap.String("port", cfg.HTTP.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("shutdown error", zap.Error(err))
	}
	log.Info("ingestion service stopped")
}
