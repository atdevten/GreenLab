// @title           GreenLab Query-Realtime API
// @version         1.0
// @description     Time-series query and real-time streaming service for the GreenLab IoT platform.
// @host            localhost:8004
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
	"encoding/pem"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"github.com/greenlab/query-realtime/internal/application"
	"github.com/greenlab/query-realtime/internal/config"
	infraInflux "github.com/greenlab/query-realtime/internal/infrastructure/influxdb"
	infraKafka "github.com/greenlab/query-realtime/internal/infrastructure/kafka"
	infraRedis "github.com/greenlab/query-realtime/internal/infrastructure/redis"
	qrHTTP "github.com/greenlab/query-realtime/internal/transport/http"
	sharedRedis "github.com/greenlab/shared/pkg/redis"

	_ "github.com/greenlab/query-realtime/docs"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}

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

	rdb := sharedRedis.Connect(log, sharedRedis.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer rdb.Close()

	reader := infraInflux.NewReader(infraInflux.Config{
		URL:    cfg.InfluxDB.URL,
		Token:  cfg.InfluxDB.Token,
		Org:    cfg.InfluxDB.Org,
		Bucket: cfg.InfluxDB.Bucket,
	})
	defer reader.Close()

	queryCache := infraRedis.NewQueryCache(rdb)
	querySvc := application.NewQueryService(
		reader, queryCache, slog.Default(),
		cfg.Query.CacheTTL, cfg.Query.LatestCacheTTL,
	)
	hub := application.NewHub(slog.Default())

	consumer := infraKafka.NewReadingConsumer(cfg.Kafka.Brokers, cfg.Kafka.GroupID, hub, slog.Default())
	defer func() {
		if err := consumer.Close(); err != nil {
			log.Error("failed to close kafka consumer", zap.Error(err))
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := consumer.Start(ctx); err != nil && err != context.Canceled {
			log.Error("kafka consumer error", zap.Error(err))
		}
	}()

	publicKey := loadPublicKey(log)

	queryHandler := qrHTTP.NewQueryHandler(querySvc, slog.Default())
	realtimeHandler := qrHTTP.NewRealtimeHandler(hub, slog.Default())
	router := qrHTTP.NewRouter(queryHandler, realtimeHandler, publicKey)

	srv := &http.Server{
		Addr:    ":" + cfg.HTTP.Port,
		Handler: router,
		// ReadTimeout and WriteTimeout are intentionally 0 because WebSocket and
		// SSE connections are long-lived. ReadHeaderTimeout is safe for all
		// connection types and protects the non-streaming endpoints from slow clients.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	go func() {
		log.Info("query-realtime service starting", zap.String("port", cfg.HTTP.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
	log.Info("query-realtime service stopped")
}

func loadPublicKey(log *zap.Logger) *rsa.PublicKey {
	path := os.Getenv("JWT_PUBLIC_KEY_PATH")
	if path == "" {
		log.Fatal("JWT_PUBLIC_KEY_PATH environment variable is not set")
	}
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
