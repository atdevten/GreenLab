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
	"github.com/gin-gonic/gin"
	"github.com/greenlab/normalization/internal/application"
	"github.com/greenlab/normalization/internal/config"
	infraInflux "github.com/greenlab/normalization/internal/infrastructure/influxdb"
	infraKafka "github.com/greenlab/normalization/internal/infrastructure/kafka"
)

func main() {
	cfg := config.Load()

	// Build the primary structured logger.
	zapLevel, err := zap.ParseAtomicLevel(cfg.Log.Level)
	if err != nil {
		zapLevel = zap.NewAtomicLevel()
	}
	zapCfg := zap.NewProductionConfig()
	zapCfg.Level = zapLevel
	log := zap.Must(zapCfg.Build())
	defer log.Sync() //nolint:errcheck

	// Bridge slog through the same zap core.
	slog.SetDefault(slog.New(zapslog.NewHandler(log.Core(), zapslog.WithCaller(true))))

	consumer := infraKafka.NewReadingConsumer(cfg.Kafka.Brokers)
	defer func() {
		if err := consumer.Close(); err != nil {
			log.Error("failed to close kafka consumer", zap.Error(err))
		}
	}()

	writer := infraInflux.NewWriter(infraInflux.Config{
		URL:    cfg.InfluxDB.URL,
		Token:  cfg.InfluxDB.Token,
		Org:    cfg.InfluxDB.Org,
		Bucket: cfg.InfluxDB.Bucket,
	})
	defer writer.Close()

	producer := infraKafka.NewNormalizedProducer(cfg.Kafka.Brokers)
	defer func() {
		if err := producer.Close(); err != nil {
			log.Error("failed to close kafka producer", zap.Error(err))
		}
	}()

	svc := application.NewNormalizationService(writer, producer, slog.Default())

	// Set up HTTP server with health endpoint.
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	srv := &http.Server{
		Addr:         ":" + cfg.HTTP.Port,
		Handler:      r,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Kafka consumer loop.
	go func() {
		log.Info("normalization consumer starting")
		for {
			evt, err := consumer.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					// Context cancelled — shutdown in progress.
					return
				}
				log.Error("failed to read message from kafka", zap.Error(err))
				continue
			}
			if err := svc.Process(ctx, evt); err != nil {
				log.Error("failed to process reading event",
					zap.String("event_id", evt.ID),
					zap.Error(err),
				)
			}
		}
	}()

	go func() {
		log.Info("normalization service starting", zap.String("port", cfg.HTTP.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down...")
	cancel() // stop consumer loop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", zap.Error(err))
	}
	log.Info("normalization service stopped")
}
