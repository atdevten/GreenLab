package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Log      LogConfig
	HTTP     HTTPConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	JWT      JWTConfig
}

type LogConfig struct {
	Level    string
	Encoding string
}

type HTTPConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
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

type JWTConfig struct {
	Issuer        string
	PublicKeyPath string
}

func Load() Config {
	return Config{
		Log: LogConfig{
			Level:    env("LOG_LEVEL", "info"),
			Encoding: "json",
		},
		HTTP: HTTPConfig{
			Port:         env("PORT", "8001"),
			ReadTimeout:  envDuration("HTTP_READ_TIMEOUT", 30*time.Second),
			WriteTimeout: envDuration("HTTP_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:  envDuration("HTTP_IDLE_TIMEOUT", 120*time.Second),
		},
		Postgres: PostgresConfig{
			DSN:             env("DSN", "postgres://greenlab:greenlab@localhost:5432/greenlab?sslmode=disable"),
			MaxOpenConns:    envInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    envInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: envDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Addr:     env("REDIS_ADDR", "localhost:6379"),
			Password: env("REDIS_PASSWORD", ""),
			DB:       envInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			Issuer:        env("JWT_ISSUER", "greenlab-identity"),
			PublicKeyPath: env("JWT_PUBLIC_KEY_PATH", "keys/public.pem"),
		},
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
