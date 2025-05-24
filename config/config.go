package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

// ServerConfig holds the server configuration
type ServerConfig struct {
	Host string
	Port int
}

// DatabaseConfig holds the database configuration
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

// KafkaConfig holds the Kafka configuration
type KafkaConfig struct {
	Brokers       []string
	ScoresTopic   string
	ConsumerGroup string
	BatchSize     int
	BatchTimeout  int // in seconds
}

// AppConfig holds the application configuration
type AppConfig struct {
	Server   ServerConfig
	Database DatabaseConfig
	Kafka    KafkaConfig
}

// NewAppConfig creates a new AppConfig from environment variables
func NewAppConfig() *AppConfig {
	return &AppConfig{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "127.0.0.1"),
			Port: getEnvAsInt("SERVER_PORT", 8080),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvAsInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			Name:     getEnv("DB_NAME", "leaderboard"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Kafka: KafkaConfig{
			Brokers:       strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ","),
			ScoresTopic:   getEnv("KAFKA_SCORES_TOPIC", "leaderboard-scores"),
			ConsumerGroup: getEnv("KAFKA_CONSUMER_GROUP", "score-processor"),
			BatchSize:     getEnvAsInt("KAFKA_BATCH_SIZE", 5000),
			BatchTimeout:  getEnvAsInt("KAFKA_BATCH_TIMEOUT", 5),
		},
	}
}

// Helper functions to get environment variables with defaults
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if valueStr, exists := os.LookupEnv(key); exists {
		if value, err := strconv.Atoi(valueStr); err == nil {
			return value
		}
		log.Printf("Warning: Environment variable %s is not a valid integer, using default", key)
	}
	return defaultValue
}
