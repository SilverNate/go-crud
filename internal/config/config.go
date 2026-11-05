package config

import (
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr        string
	StorageDriver   string
	DatabaseURL     string
	LogLevel        string
	WorkerQueueSize int
}

func Load() Config {
	return Config{
		HTTPAddr:        getEnv("HTTP_ADDR", ":8080"),
		StorageDriver:   getEnv("STORAGE_DRIVER", "memory"),
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		LogLevel:        getEnv("LOG_LEVEL", "debug"),
		WorkerQueueSize: getInt("WORKER_QUEUE_SIZE", 64),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}
