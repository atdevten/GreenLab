package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Log      LogConfig
	HTTP     HTTPConfig
	Kafka    KafkaConfig
	InfluxDB InfluxDBConfig
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

type KafkaConfig struct {
	Brokers []string
}

type InfluxDBConfig struct {
	URL    string
	Token  string
	Org    string
	Bucket string
}

func Load() Config {
	return Config{
		Log: LogConfig{
			Level:    env("LOG_LEVEL", "info"),
			Encoding: "json",
		},
		HTTP: HTTPConfig{
			Port:            env("PORT", "8006"),
			ReadTimeout:     envDuration("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:    envDuration("HTTP_WRITE_TIMEOUT", 10*time.Second),
			IdleTimeout:     envDuration("HTTP_IDLE_TIMEOUT", 120*time.Second),
			ShutdownTimeout: envDuration("HTTP_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Kafka: KafkaConfig{
			Brokers: parseBrokers(env("KAFKA_BROKERS", "localhost:9092")),
		},
		InfluxDB: InfluxDBConfig{
			URL:    env("INFLUXDB_URL", "http://localhost:8086"),
			Token:  requiredEnv("INFLUXDB_TOKEN"),
			Org:    env("INFLUXDB_ORG", "greenlab"),
			Bucket: env("INFLUXDB_BUCKET", "telemetry"),
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
