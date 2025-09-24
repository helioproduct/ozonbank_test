package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Postgres    PostgresConfig
	WS          WSConfig
	HTTP        HTTPConfig
	StorageType string
}

type PostgresConfig struct {
	User     string
	Password string
	DB       string
	Host     string
	Port     int
	SSLMode  string
}

func (pc PostgresConfig) GetDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		pc.User,
		pc.Password,
		pc.Host,
		pc.Port,
		pc.DB,
		pc.SSLMode,
	)
}

type HTTPConfig struct {
	Port string
}

type WSConfig struct {
	KeepAliveSeconds int
}

func LoadConfig() Config {
	storageType := mustGetEnv("STORAGE_TYPE")

	cfg := Config{
		StorageType: storageType,
		HTTP: HTTPConfig{
			Port: mustGetEnv("HTTP_PORT"),
		},
		WS: WSConfig{
			KeepAliveSeconds: mustGetInt("WS_KEEPALIVE_SECONDS"),
		},
	}

	if storageType == "postgres" {
		cfg.Postgres = PostgresConfig{
			User:     mustGetEnv("POSTGRES_USER"),
			Password: mustGetEnv("POSTGRES_PASSWORD"),
			DB:       mustGetEnv("POSTGRES_DB"),
			Host:     mustGetEnv("POSTGRES_HOST"),
			Port:     mustGetInt("POSTGRES_PORT"),
			SSLMode:  mustGetEnv("POSTGRES_SSLMODE"),
		}
	}

	return cfg
}

func mustGetEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic("missing required env var: " + key)
	}
	return val
}

func mustGetInt(key string) int {
	val := mustGetEnv(key)
	i, err := strconv.Atoi(val)
	if err != nil {
		panic("invalid int for env var " + key + ": " + val)
	}
	return i
}
