package config

import (
	"fmt"
	"net/url"
	"strings"
)

// validate validates the configuration
func validate(config *Config) error {
	// Validate server configuration
	if err := validateServer(&config.Server); err != nil {
		return fmt.Errorf("server config: %w", err)
	}

	// Validate web configuration
	if err := validateWeb(&config.Web); err != nil {
		return fmt.Errorf("web config: %w", err)
	}

	// Validate bridge configurations
	for i, bridge := range config.Bridges {
		if err := validateBridge(&bridge); err != nil {
			return fmt.Errorf("bridge config[%d]: %w", i, err)
		}
	}

	// Validate MQTT configuration
	if err := validateMQTT(&config.MQTT); err != nil {
		return fmt.Errorf("mqtt config: %w", err)
	}

	// Validate logging configuration
	if err := validateLogging(&config.Logging); err != nil {
		return fmt.Errorf("logging config: %w", err)
	}

	// Validate metrics configuration
	if err := validateMetrics(&config.Metrics); err != nil {
		return fmt.Errorf("metrics config: %w", err)
	}

	return nil
}

// validateServer validates server configuration
func validateServer(config *ServerConfig) error {
	if config.Port < 1 || config.Port > 65535 {
		return fmt.Errorf("invalid port: %d", config.Port)
	}

	if config.MaxConnections < 1 {
		return fmt.Errorf("max_connections must be at least 1")
	}

	if config.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	if config.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	if len(config.Name) > 16 {
		return fmt.Errorf("name too long (max 16 characters): %s", config.Name)
	}

	if len(config.Description) > 14 {
		return fmt.Errorf("description too long (max 14 characters): %s", config.Description)
	}

	if config.TalkMaxDuration <= 0 {
		return fmt.Errorf("talk_max_duration must be positive")
	}

	if config.UnmuteAfter < 0 {
		return fmt.Errorf("unmute_after cannot be negative")
	}

	return nil
}

// validateWeb validates web configuration
func validateWeb(config *WebConfig) error {
	if !config.Enabled {
		return nil
	}

	if config.Port < 1 || config.Port > 65535 {
		return fmt.Errorf("invalid port: %d", config.Port)
	}

	if config.AuthRequired {
		if config.Username == "" {
			return fmt.Errorf("username required when auth is enabled")
		}
		if config.Password == "" {
			return fmt.Errorf("password required when auth is enabled")
		}
	}

	return nil
}

// validateBridge validates bridge configuration
func validateBridge(config *BridgeConfig) error {
	if !config.Enabled {
		return nil
	}

	if config.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	if config.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}

	if config.Port < 1 || config.Port > 65535 {
		return fmt.Errorf("invalid port: %d", config.Port)
	}

	// Permanent bridges don't need schedule/duration
	if !config.Permanent {
		if config.Schedule == "" {
			return fmt.Errorf("schedule cannot be empty for non-permanent bridge")
		}

		if config.Duration <= 0 {
			return fmt.Errorf("duration must be positive for scheduled bridge")
		}
	}

	// Validate retry configuration
	if config.RetryDelay < 0 {
		return fmt.Errorf("retry_delay cannot be negative")
	}

	if config.HealthCheck < 0 {
		return fmt.Errorf("health_check cannot be negative")
	}

	return nil
}

// validateMQTT validates MQTT configuration
func validateMQTT(config *MQTTConfig) error {
	if !config.Enabled {
		return nil
	}

	if config.Broker == "" {
		return fmt.Errorf("broker cannot be empty")
	}

	// Validate broker URL
	if _, err := url.Parse(config.Broker); err != nil {
		return fmt.Errorf("invalid broker URL: %w", err)
	}

	if config.ClientID == "" {
		return fmt.Errorf("client_id cannot be empty")
	}

	if config.TopicPrefix == "" {
		return fmt.Errorf("topic_prefix cannot be empty")
	}

	if config.QoS > 2 {
		return fmt.Errorf("invalid QoS level: %d (must be 0, 1, or 2)", config.QoS)
	}

	return nil
}

// validateLogging validates logging configuration
func validateLogging(config *LoggingConfig) error {
	validLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLevels, config.Level) {
		return fmt.Errorf("invalid log level: %s (must be one of: %s)",
			config.Level, strings.Join(validLevels, ", "))
	}

	validFormats := []string{"text", "json"}
	if !contains(validFormats, config.Format) {
		return fmt.Errorf("invalid log format: %s (must be one of: %s)",
			config.Format, strings.Join(validFormats, ", "))
	}

	if config.MaxSize < 1 {
		return fmt.Errorf("max_size must be at least 1")
	}

	if config.MaxBackups < 0 {
		return fmt.Errorf("max_backups cannot be negative")
	}

	if config.MaxAge < 0 {
		return fmt.Errorf("max_age cannot be negative")
	}

	return nil
}

// validateMetrics validates metrics configuration
func validateMetrics(config *MetricsConfig) error {
	if !config.Enabled {
		return nil
	}

	if config.Prometheus.Enabled {
		if config.Prometheus.Port < 1 || config.Prometheus.Port > 65535 {
			return fmt.Errorf("invalid prometheus port: %d", config.Prometheus.Port)
		}

		if config.Prometheus.Path == "" {
			return fmt.Errorf("prometheus path cannot be empty")
		}

		if !strings.HasPrefix(config.Prometheus.Path, "/") {
			return fmt.Errorf("prometheus path must start with /")
		}
	}

	return nil
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
