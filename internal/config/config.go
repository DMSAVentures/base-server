package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

var ErrEmptyEnvironmentVariable = errors.New("empty environment variable")

// Config holds all application configuration
type Config struct {
	Database   DatabaseConfig
	Auth       AuthConfig
	Services   ServicesConfig
	Kafka      KafkaConfig
	WorkerPool WorkerPoolConfig
	Server     ServerConfig
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	Host     string
	Username string
	Password string
	Name     string
}

// AuthConfig holds authentication-related configuration
type AuthConfig struct {
	JWTSecret          string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURI  string
}

// ServicesConfig holds external service API keys and configuration
type ServicesConfig struct {
	StripeSecretKey         string
	StripeWebhookSecret     string
	ResendAPIKey            string
	DefaultEmailSender      string
	GoogleAIAPIKey          string
	OpenAIAPIKey            string
	WebAppURI               string
}

// KafkaConfig holds Kafka/event streaming configuration
type KafkaConfig struct {
	Brokers       string
	Topic         string
	ConsumerGroup string
}

// WorkerPoolConfig holds worker pool configuration for event processing
type WorkerPoolConfig struct {
	WebhookWorkers  int // Number of workers for webhook event processing
	EmailWorkers    int // Number of workers for email event processing
	PositionWorkers int // Number of workers for position calculation event processing
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port int
}

// Load reads and validates all required environment variables
func Load() (*Config, error) {
	// Load env.local in non-production environments
	if os.Getenv("GO_ENV") != "production" {
		if err := godotenv.Load("env.local"); err != nil {
			return nil, fmt.Errorf("failed to load env.local: %w", err)
		}
	}

	cfg := &Config{}

	// Database configuration
	var err error
	if cfg.Database.Host, err = requireEnv("DB_HOST"); err != nil {
		return nil, err
	}
	if cfg.Database.Username, err = requireEnv("DB_USERNAME"); err != nil {
		return nil, err
	}
	if cfg.Database.Password, err = requireEnv("DB_PASSWORD"); err != nil {
		return nil, err
	}
	if cfg.Database.Name, err = requireEnv("DB_NAME"); err != nil {
		return nil, err
	}

	// Auth configuration
	if cfg.Auth.JWTSecret, err = requireEnv("JWT_SECRET"); err != nil {
		return nil, err
	}
	if cfg.Auth.GoogleClientID, err = requireEnv("GOOGLE_CLIENT_ID"); err != nil {
		return nil, err
	}
	if cfg.Auth.GoogleClientSecret, err = requireEnv("GOOGLE_CLIENT_SECRET"); err != nil {
		return nil, err
	}
	if cfg.Auth.GoogleRedirectURI, err = requireEnv("GOOGLE_REDIRECT_URI"); err != nil {
		return nil, err
	}

	// Services configuration
	if cfg.Services.StripeSecretKey, err = requireEnv("STRIPE_SECRET_KEY"); err != nil {
		return nil, err
	}
	if cfg.Services.StripeWebhookSecret, err = requireEnv("STRIPE_WEBHOOK_SECRET"); err != nil {
		return nil, err
	}
	if cfg.Services.ResendAPIKey, err = requireEnv("RESEND_API_KEY"); err != nil {
		return nil, err
	}
	if cfg.Services.DefaultEmailSender, err = requireEnv("DEFAULT_EMAIL_SENDER_ADDRESS"); err != nil {
		return nil, err
	}
	if cfg.Services.GoogleAIAPIKey, err = requireEnv("GOOGLE_AI_API_KEY"); err != nil {
		return nil, err
	}
	if cfg.Services.OpenAIAPIKey, err = requireEnv("OPENAI_API_KEY"); err != nil {
		return nil, err
	}
	if cfg.Services.WebAppURI, err = requireEnv("WEBAPP_URI"); err != nil {
		return nil, err
	}

	// Kafka configuration
	if cfg.Kafka.Brokers, err = requireEnv("KAFKA_BROKERS"); err != nil {
		return nil, err
	}
	cfg.Kafka.Topic = getEnvWithDefault("KAFKA_TOPIC", "webhook-events")
	cfg.Kafka.ConsumerGroup = getEnvWithDefault("KAFKA_CONSUMER_GROUP", "webhook-consumers")

	// Worker pool configuration
	webhookWorkers := getEnvWithDefault("WEBHOOK_WORKERS", "10")
	cfg.WorkerPool.WebhookWorkers, err = strconv.Atoi(webhookWorkers)
	if err != nil {
		return nil, fmt.Errorf("failed to parse WEBHOOK_WORKERS: %w", err)
	}

	emailWorkers := getEnvWithDefault("EMAIL_WORKERS", "5")
	cfg.WorkerPool.EmailWorkers, err = strconv.Atoi(emailWorkers)
	if err != nil {
		return nil, fmt.Errorf("failed to parse EMAIL_WORKERS: %w", err)
	}

	positionWorkers := getEnvWithDefault("POSITION_WORKERS", "3")
	cfg.WorkerPool.PositionWorkers, err = strconv.Atoi(positionWorkers)
	if err != nil {
		return nil, fmt.Errorf("failed to parse POSITION_WORKERS: %w", err)
	}

	// Server configuration
	serverPort, err := requireEnv("SERVER_PORT")
	if err != nil {
		return nil, err
	}
	cfg.Server.Port, err = strconv.Atoi(serverPort)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SERVER_PORT: %w", err)
	}

	return cfg, nil
}

// ConnectionString returns a PostgreSQL connection string
func (c *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf("postgres://%s:%s@%s/%s",
		c.Username, c.Password, c.Host, c.Name)
}

// requireEnv retrieves an environment variable or returns an error if empty
func requireEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("%s is not set: %w", key, ErrEmptyEnvironmentVariable)
	}
	return value, nil
}

// getEnvWithDefault retrieves an environment variable or returns a default value
func getEnvWithDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
