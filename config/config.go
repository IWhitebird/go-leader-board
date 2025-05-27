package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
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
	Brokers           []string
	ScoresTopicPrefix string // Topic name for scores
	ConsumerGroup     string
	BatchSize         int
	BatchTimeout      int    // in seconds
	ServiceID         string // Unique identifier for this service instance
}

// AppConfig holds the application configuration
type AppConfig struct {
	Server   ServerConfig
	Database DatabaseConfig
	Kafka    KafkaConfig
}

// NewAppConfig creates a new AppConfig from environment variables
func NewAppConfig() *AppConfig {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}
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
			Brokers:           strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ","),
			ScoresTopicPrefix: getEnv("KAFKA_SCORES_TOPIC_PREFIX", "leaderboard-scores"),
			ConsumerGroup:     getEnv("KAFKA_CONSUMER_GROUP", "score-processor"),
			BatchSize:         getEnvAsInt("KAFKA_BATCH_SIZE", 5000),
			BatchTimeout:      getEnvAsInt("KAFKA_BATCH_TIMEOUT", 5),
			ServiceID:         generateServiceID(),
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

// generateServiceID creates a unique service ID for this instance
func generateServiceID() string {
	// First try to get from environment (for Docker containers)
	if serviceID := getEnv("SERVICE_ID", ""); serviceID != "" {
		return serviceID
	}

	// Try to get hostname
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		return hostname
	}

	// Fallback to timestamp-based ID
	return fmt.Sprintf("service-%d", time.Now().UnixNano())
}
