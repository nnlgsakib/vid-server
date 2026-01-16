package main

import (
	"fmt"
	"os"
	"strconv"
)

// LoadConfig loads configuration from environment variables or uses defaults
func LoadConfig() *Config {
	config := &Config{
		ServerPort:    getEnvOrDefault("SERVER_PORT", "8080"),
		StoragePath:   getEnvOrDefault("STORAGE_PATH", "./storage"),
		MaxFileSize:   parseInt64EnvOrDefault("MAX_FILE_SIZE", 1024*1024*500), // 500MB
		EnableLogging: getEnvOrDefault("ENABLE_LOGGING", "true") == "true",
	}
	
	return config
}

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// parseInt64EnvOrDefault returns the value of an environment variable parsed as int64 or a default value
func parseInt64EnvOrDefault(key string, defaultValue int64) int64 {
	if valueStr := os.Getenv(key); valueStr != "" {
		if value, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
			return value
		}
		fmt.Printf("Warning: Invalid value for %s, using default\n", key)
	}
	return defaultValue
}