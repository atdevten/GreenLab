package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Log        LogConfig
	HTTP       HTTPConfig
	Postgres   PostgresConfig
	Kafka      KafkaConfig
	SMTP       SMTPConfig
	RuleEngine RuleEngineConfig
	JWT        JWTConfig
}

type LogConfig struct {
	Level    string
	Encoding string
}

type HTTPConfig struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

type PostgresConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type KafkaConfig struct {
	Brokers          []string
	TelemetryGroupID string
	AlertGroupID     string
}

type SMTPConfig struct {
	Host              string
	Port              string
	Username          string
	Password          string
	From              string
	FallbackRecipient string
}

type RuleEngineConfig struct {
	RefreshInterval time.Duration
}

type JWTConfig struct {
	PublicKeyPath string
}

// Load reads configuration from environment variables and returns an error if
// any required variable is absent.
func Load() (Config, error) {
	dsn, err := requiredEnv("DSN")
	if err != nil {
		return Config{}, err
	}
	jwtKeyPath, err := requiredEnv("JWT_PUBLIC_KEY_PATH")
	if err != nil {
		return Config{}, err
	}

	return Config{
		Log: LogConfig{
			Level:    env("LOG_LEVEL", "info"),
			Encoding: "json",
		},
		HTTP: HTTPConfig{
			Port:            env("PORT", "8005"),
			ReadTimeout:     envDuration("HTTP_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:    envDuration("HTTP_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:     envDuration("HTTP_IDLE_TIMEOUT", 120*time.Second),
			ShutdownTimeout: envDuration("HTTP_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Postgres: PostgresConfig{
			DSN:             dsn,
			MaxOpenConns:    envInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    envInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: envDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Kafka: KafkaConfig{
			Brokers:          parseBrokers(env("KAFKA_BROKERS", "localhost:9092")),
			TelemetryGroupID: env("KAFKA_TELEMETRY_GROUP_ID", "alert-notification-telemetry-group"),
			AlertGroupID:     env("KAFKA_ALERT_GROUP_ID", "alert-notification-alert-group"),
		},
		SMTP: SMTPConfig{
			Host:              env("SMTP_HOST", "smtp.example.com"),
			Port:              env("SMTP_PORT", "587"),
			Username:          env("SMTP_USERNAME", ""),
			Password:          env("SMTP_PASSWORD", ""),
			From:              env("SMTP_FROM", "noreply@greenlab.io"),
			FallbackRecipient: env("ALERT_FALLBACK_RECIPIENT", "alerts@greenlab.io"),
		},
		RuleEngine: RuleEngineConfig{
			RefreshInterval: envDuration("RULE_REFRESH_INTERVAL", 60*time.Second),
		},
		JWT: JWTConfig{
			PublicKeyPath: jwtKeyPath,
		},
	}, nil
}

// parseBrokers splits a comma-separated broker list and trims whitespace from each entry.
func parseBrokers(s string) []string {
	parts := strings.Split(s, ",")
	for i, b := range parts {
		parts[i] = strings.TrimSpace(b)
	}
	return parts
}

// requiredEnv returns the value of key or an error if it is unset or empty.
func requiredEnv(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("required environment variable %q is not set", key)
	}
	return v, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: %q=%q is not a valid integer, using default %d\n", key, v, fallback)
		return fallback
	}
	return n
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: %q=%q is not a valid duration, using default %s\n", key, v, fallback)
		return fallback
	}
	return d
}
