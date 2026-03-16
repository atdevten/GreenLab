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
	Redis    RedisConfig
	Kafka    KafkaConfig
	InfluxDB InfluxDBConfig
	Query    QueryConfig
}

type LogConfig struct {
	Level    string
	Encoding string
}

type HTTPConfig struct {
	Port            string
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type KafkaConfig struct {
	Brokers []string
	GroupID string
}

type InfluxDBConfig struct {
	URL    string
	Token  string
	Org    string
	Bucket string
}

type QueryConfig struct {
	// CacheTTL controls how long time-series query results are cached.
	CacheTTL time.Duration
	// LatestCacheTTL controls how long the most-recent-value cache entry lives.
	LatestCacheTTL time.Duration
}

// Load reads configuration from environment variables and returns an error if
// any required variable is absent. Callers (main) should treat a non-nil error
// as fatal; os.Exit must not be called inside this package so that tests remain
// straightforward.
func Load() (Config, error) {
	influxToken, err := requiredEnv("INFLUXDB_TOKEN")
	if err != nil {
		return Config{}, err
	}

	return Config{
		Log: LogConfig{
			Level:    env("LOG_LEVEL", "info"),
			Encoding: "json",
		},
		HTTP: HTTPConfig{
			Port:            env("PORT", "8004"),
			IdleTimeout:     envDuration("HTTP_IDLE_TIMEOUT", 120*time.Second),
			ShutdownTimeout: envDuration("HTTP_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Redis: RedisConfig{
			Addr:     env("REDIS_ADDR", "localhost:6379"),
			Password: env("REDIS_PASSWORD", ""),
			DB:       envInt("REDIS_DB", 0),
		},
		Kafka: KafkaConfig{
			Brokers: parseBrokers(env("KAFKA_BROKERS", "localhost:9092")),
			GroupID: env("KAFKA_GROUP_ID", "query-realtime-group"),
		},
		InfluxDB: InfluxDBConfig{
			URL:    env("INFLUXDB_URL", "http://localhost:8086"),
			Token:  influxToken,
			Org:    env("INFLUXDB_ORG", "greenlab"),
			Bucket: env("INFLUXDB_BUCKET", "telemetry"),
		},
		Query: QueryConfig{
			CacheTTL:       envDuration("QUERY_CACHE_TTL", 30*time.Second),
			LatestCacheTTL: envDuration("LATEST_CACHE_TTL", 5*time.Second),
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
