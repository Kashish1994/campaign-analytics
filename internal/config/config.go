package config

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Init initializes the configuration
func Init() error {
	// Set defaults
	setDefaults()

	// Look for config files
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("../config")
	viper.AddConfigPath("../../config")
	viper.AddConfigPath("/etc/campaign-analytics")
	viper.AddConfigPath("$HOME/.campaign-analytics")

	// Read environment variables
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetEnvPrefix("CA")

	// Read the config file
	if err := viper.ReadInConfig(); err != nil {
		// Config file not found, use default values and environment variables
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		log.Println("No config file found, using defaults and environment variables")
	} else {
		log.Printf("Using config file: %s\n", viper.ConfigFileUsed())
	}

	// Validate required settings
	if err := validateConfig(); err != nil {
		return err
	}

	return nil
}

// setDefaults sets the default configuration values
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.read_timeout", 10*time.Second)
	viper.SetDefault("server.write_timeout", 30*time.Second)
	viper.SetDefault("server.idle_timeout", 120*time.Second)

	// Database defaults
	viper.SetDefault("postgres.host", "localhost")
	viper.SetDefault("postgres.port", 5432)
	viper.SetDefault("postgres.database", "campaign_analytics")
	viper.SetDefault("postgres.sslmode", "disable")
	viper.SetDefault("postgres.max_open_conns", 25)
	viper.SetDefault("postgres.max_idle_conns", 5)
	viper.SetDefault("postgres.conn_max_lifetime", 5*time.Minute)

	viper.SetDefault("clickhouse.host", "localhost")
	viper.SetDefault("clickhouse.port", 9000)
	viper.SetDefault("clickhouse.database", "campaign_analytics")
	viper.SetDefault("clickhouse.max_open_conns", 50)
	viper.SetDefault("clickhouse.max_idle_conns", 10)
	viper.SetDefault("clickhouse.conn_max_lifetime", 1*time.Hour)

	// Redis defaults
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 10)
	viper.SetDefault("redis.min_idle_conns", 5)

	// Kafka defaults
	viper.SetDefault("kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("kafka.consumer.group_id", "campaign-analytics-consumer")
	viper.SetDefault("kafka.producer.require_acks", "all")
	viper.SetDefault("kafka.producer.max_attempts", 10)

	// JWT defaults
	viper.SetDefault("jwt.key", "default-jwt-secret-key-change-in-production")
	viper.SetDefault("jwt.expiration", 24*time.Hour)

	// Rate limiting defaults
	viper.SetDefault("rate_limiting.default_rate", 100) // per minute
	viper.SetDefault("rate_limiting.heavy_rate", 20)    // per minute

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.development", false)
	viper.SetDefault("logging.encoding", "json")
}

// validateConfig validates the required configuration settings
func validateConfig() error {
	// List of settings to validate as non-empty
	requiredSettings := []string{
		"server.port",
		"jwt.key",
	}

	for _, setting := range requiredSettings {
		if viper.GetString(setting) == "" {
			return fmt.Errorf("required configuration setting %s is empty", setting)
		}
	}

	return nil
}
