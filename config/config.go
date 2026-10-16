package config

import (
	"os"
	"sync"
)

var (
	testConfig *Config
	mu       sync.Mutex
)

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	JWTSecret  string
	ServerPort string
}

func Load() *Config {
	mu.Lock()
	defer mu.Unlock()
	if testConfig != nil {
		return testConfig
	}
	return &Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "forum"),
		JWTSecret:  getEnv("JWT_SECRET", "change-this-secret"),
		ServerPort: getEnv("SERVER_PORT", "8080"),
	}
}

func SetForTesting(cfg *Config) {
	mu.Lock()
	defer mu.Unlock()
	testConfig = cfg
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
