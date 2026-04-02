// @title           GreenLab Alert-Notification API
// @version         1.0
// @description     Alert rule management and notification dispatch for the GreenLab IoT platform.
// @host            localhost:8005
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

	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"github.com/greenlab/alert-notification/internal/application"
	"github.com/greenlab/alert-notification/internal/config"
	infraEmail "github.com/greenlab/alert-notification/internal/infrastructure/email"
	infraKafka "github.com/greenlab/alert-notification/internal/infrastructure/kafka"
	infraPostgres "github.com/greenlab/alert-notification/internal/infrastructure/postgres"
	infraWebhook "github.com/greenlab/alert-notification/internal/infrastructure/webhook"
	anHTTP "github.com/greenlab/alert-notification/internal/transport/http"
	sharedPostgres "github.com/greenlab/shared/pkg/postgres"

	_ "github.com/greenlab/alert-notification/docs"
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

	db := sharedPostgres.Connect(log, sharedPostgres.Config{
		DSN:             cfg.Postgres.DSN,
		MaxOpenConns:    cfg.Postgres.MaxOpenConns,
		MaxIdleConns:    cfg.Postgres.MaxIdleConns,
		ConnMaxLifetime: cfg.Postgres.ConnMaxLifetime,
	})
	defer db.Close()
	mustCheckSchema(db.DB, log)

	// Alert dependencies
	ruleRepo := infraPostgres.NewRuleRepo(db)
	deliveryRepo := infraPostgres.NewDeliveryRepo(db)
	alertProducer := infraKafka.NewAlertProducer(cfg.Kafka.Brokers)
	defer func() {
		if err := alertProducer.Close(); err != nil {
			log.Error("failed to close alert producer", zap.Error(err))
		}
	}()

	engine := application.NewRuleEngine(ruleRepo, alertProducer, slog.Default())
	alertSvc := application.NewAlertService(ruleRepo, alertProducer, deliveryRepo, slog.Default())

	// Notification dependencies
	emailSender := infraEmail.NewSMTPSender(infraEmail.Config{
		Host:     cfg.SMTP.Host,
		Port:     cfg.SMTP.Port,
		Username: cfg.SMTP.Username,
		Password: cfg.SMTP.Password,
		From:     cfg.SMTP.From,
	})
	webhookClient := infraWebhook.NewClient()
	dispatcher := application.NewDispatcher(emailSender, webhookClient, deliveryRepo, slog.Default())

	notifRepo := infraPostgres.NewNotificationRepo(db)
	notifSvc := application.NewNotificationService(notifRepo, dispatcher, slog.Default(), cfg.SMTP.FallbackRecipient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load rules initially; a failure is non-fatal — the engine starts empty
	// and will be populated on the next refresh tick.
	if err := engine.LoadRules(ctx); err != nil {
		log.Error("initial rule load failed", zap.Error(err))
	}

	go engine.StartRuleRefresh(ctx, cfg.RuleEngine.RefreshInterval)

	// Start telemetry consumer (normalized.sensor → rule engine)
	telemetryConsumer := infraKafka.NewTelemetryConsumer(cfg.Kafka.Brokers, cfg.Kafka.TelemetryGroupID, engine, slog.Default())
	defer func() {
		if err := telemetryConsumer.Close(); err != nil {
			log.Error("failed to close telemetry consumer", zap.Error(err))
		}
	}()
	go func() {
		if err := telemetryConsumer.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("telemetry consumer error", zap.Error(err))
		}
	}()

	// Start alert consumer (alert.events → notifications)
	alertConsumer := infraKafka.NewAlertConsumer(cfg.Kafka.Brokers, cfg.Kafka.AlertGroupID, notifSvc, slog.Default())
	defer func() {
		if err := alertConsumer.Close(); err != nil {
			log.Error("failed to close alert consumer", zap.Error(err))
		}
	}()
	go func() {
		if err := alertConsumer.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("alert consumer error", zap.Error(err))
		}
	}()

	publicKey := loadPublicKey(log, cfg.JWT.PublicKeyPath)

	alertHandler := anHTTP.NewAlertHandler(alertSvc, slog.Default())
	notifHandler := anHTTP.NewNotificationHandler(notifSvc, slog.Default())
	router := anHTTP.NewRouter(alertHandler, notifHandler, publicKey)

	srv := &http.Server{
		Addr:         ":" + cfg.HTTP.Port,
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	go func() {
		log.Info("alert-notification service starting", zap.String("port", cfg.HTTP.Port))
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
	log.Info("alert-notification service stopped")
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
