package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Log      LogConfig
	HTTP     HTTPConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	Kafka    KafkaConfig
	Ingest   IngestConfig
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

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type KafkaConfig struct {
	Brokers []string
}

type IngestConfig struct {
	// MaxReadingAge bounds how far in the past a client-supplied timestamp may be.
	// Pass 0 (or unset MAX_READING_AGE) to disable the past-bound check.
	MaxReadingAge time.Duration
}

func Load() Config {
	return Config{
		Log: LogConfig{
			Level:    env("LOG_LEVEL", "info"),
			Encoding: "json",
		},
		HTTP: HTTPConfig{
			Port:            env("PORT", "8003"),
			ReadTimeout:     envDuration("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:    envDuration("HTTP_WRITE_TIMEOUT", 10*time.Second),
			IdleTimeout:     envDuration("HTTP_IDLE_TIMEOUT", 120*time.Second),
			ShutdownTimeout: envDuration("HTTP_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Postgres: PostgresConfig{
			DSN:             requiredEnv("DSN"),
			MaxOpenConns:    envInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    envInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: envDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Addr:     env("REDIS_ADDR", "localhost:6379"),
			Password: env("REDIS_PASSWORD", ""),
			DB:       envInt("REDIS_DB", 0),
		},
		Kafka: KafkaConfig{
			Brokers: parseBrokers(env("KAFKA_BROKERS", "localhost:9092")),
		},
		Ingest: IngestConfig{
			MaxReadingAge: envDuration("MAX_READING_AGE", 24*time.Hour),
		},
	}
}

// parseBrokers splits a comma-separated broker list and trims whitespace from each entry.
func parseBrokers(s string) []string {
	parts := strings.Split(s, ",")
	for i, b := range parts {
		parts[i] = strings.TrimSpace(b)
	}
	return parts
}

// requiredEnv returns the value of key or exits if it is unset or empty.
func requiredEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "FATAL: required environment variable %q is not set\n", key)
		os.Exit(1)
	}
	return v
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
