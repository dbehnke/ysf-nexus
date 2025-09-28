package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Web       WebConfig       `mapstructure:"web"`
	Bridges   []BridgeConfig  `mapstructure:"bridges"`
	MQTT      MQTTConfig      `mapstructure:"mqtt"`
	Blocklist BlocklistConfig `mapstructure:"blocklist"`
	Logging   LoggingConfig   `mapstructure:"logging"`
	Metrics   MetricsConfig   `mapstructure:"metrics"`
}

// ServerConfig holds YSF server configuration
type ServerConfig struct {
	Host           string        `mapstructure:"host"`
	Port           int           `mapstructure:"port"`
	Timeout        time.Duration `mapstructure:"timeout"`
	MaxConnections int           `mapstructure:"max_connections"`
	Name           string        `mapstructure:"name"`
	Description    string        `mapstructure:"description"`
}

// WebConfig holds web dashboard configuration
type WebConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	AuthRequired bool   `mapstructure:"auth_required"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
}

// BridgeConfig holds bridge connection configuration
type BridgeConfig struct {
	Name     string        `mapstructure:"name"`
	Host     string        `mapstructure:"host"`
	Port     int           `mapstructure:"port"`
	Schedule string        `mapstructure:"schedule"`
	Duration time.Duration `mapstructure:"duration"`
	Enabled  bool          `mapstructure:"enabled"`
}

// MQTTConfig holds MQTT client configuration
type MQTTConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	Broker      string `mapstructure:"broker"`
	TopicPrefix string `mapstructure:"topic_prefix"`
	ClientID    string `mapstructure:"client_id"`
	Username    string `mapstructure:"username"`
	Password    string `mapstructure:"password"`
	QoS         byte   `mapstructure:"qos"`
	Retained    bool   `mapstructure:"retained"`
}

// BlocklistConfig holds blocklist configuration
type BlocklistConfig struct {
	Enabled   bool     `mapstructure:"enabled"`
	Callsigns []string `mapstructure:"callsigns"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	File       string `mapstructure:"file"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
}

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	Enabled    bool             `mapstructure:"enabled"`
	Prometheus PrometheusConfig `mapstructure:"prometheus"`
}

// PrometheusConfig holds Prometheus metrics configuration
type PrometheusConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Port    int    `mapstructure:"port"`
	Path    string `mapstructure:"path"`
}

// Load loads configuration from file and environment variables
func Load(configFile string) (*Config, error) {
	// Set defaults
	setDefaults()

	// Set config file
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./configs")
		viper.AddConfigPath("/etc/ysf-nexus")
	}

	// Environment variables
	viper.SetEnvPrefix("YSF")
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found is OK, use defaults
		} else if os.IsNotExist(err) {
			// File explicitly specified but doesn't exist - that's also OK
		} else {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal to struct
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := validate(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 42000)
	viper.SetDefault("server.timeout", "5m")
	viper.SetDefault("server.max_connections", 200)
	viper.SetDefault("server.name", "YSF Nexus")
	viper.SetDefault("server.description", "Go Reflector")

	// Web defaults
	viper.SetDefault("web.enabled", true)
	viper.SetDefault("web.host", "0.0.0.0")
	viper.SetDefault("web.port", 8080)
	viper.SetDefault("web.auth_required", false)

	// MQTT defaults
	viper.SetDefault("mqtt.enabled", false)
	viper.SetDefault("mqtt.broker", "tcp://localhost:1883")
	viper.SetDefault("mqtt.topic_prefix", "ysf/reflector")
	viper.SetDefault("mqtt.client_id", "ysf-nexus")
	viper.SetDefault("mqtt.qos", 1)
	viper.SetDefault("mqtt.retained", false)

	// Blocklist defaults
	viper.SetDefault("blocklist.enabled", true)

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "text")
	viper.SetDefault("logging.max_size", 100)
	viper.SetDefault("logging.max_backups", 3)
	viper.SetDefault("logging.max_age", 28)

	// Metrics defaults
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.prometheus.enabled", true)
	viper.SetDefault("metrics.prometheus.port", 9090)
	viper.SetDefault("metrics.prometheus.path", "/metrics")
}
