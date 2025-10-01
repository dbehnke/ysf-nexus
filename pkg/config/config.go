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
	YSF2DMR   YSF2DMRConfig   `mapstructure:"ysf2dmr"`
}

// ServerConfig holds YSF server configuration
type ServerConfig struct {
	Host           string        `mapstructure:"host"`
	Port           int           `mapstructure:"port"`
	Timeout        time.Duration `mapstructure:"timeout"`
	MaxConnections int           `mapstructure:"max_connections"`
	Name           string        `mapstructure:"name"`
	Description    string        `mapstructure:"description"`
	// TalkMaxDuration is the maximum continuous talk duration before muting a repeater
	TalkMaxDuration time.Duration `mapstructure:"talk_max_duration"`
	// UnmuteAfter is the duration after which a muted repeater will be automatically unmuted
	// If zero, muted repeaters remain muted until they stop talking
	UnmuteAfter time.Duration `mapstructure:"unmute_after"`
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
	Name        string        `mapstructure:"name"`
	Type        string        `mapstructure:"type"` // Bridge type: "ysf" or "dmr" (default: "ysf")
	Enabled     bool          `mapstructure:"enabled"`
	Schedule    string        `mapstructure:"schedule"`     // Cron schedule (empty for permanent)
	Duration    time.Duration `mapstructure:"duration"`     // Duration for scheduled bridges
	Permanent   bool          `mapstructure:"permanent"`    // If true, ignore schedule and stay connected always
	MaxRetries  int           `mapstructure:"max_retries"`  // Max reconnection attempts (0 = infinite)
	RetryDelay  time.Duration `mapstructure:"retry_delay"`  // Initial retry delay for exponential backoff
	HealthCheck time.Duration `mapstructure:"health_check"` // How often to check connection health

	// YSF bridge fields (used when type="ysf")
	Host string `mapstructure:"host"` // Remote YSF reflector host
	Port int    `mapstructure:"port"` // Remote YSF reflector port

	// DMR bridge fields (used when type="dmr")
	DMR *DMRBridgeConfig `mapstructure:"dmr"` // DMR-specific configuration
}

// DMRBridgeConfig holds DMR-specific bridge configuration
type DMRBridgeConfig struct {
	ID                uint32        `mapstructure:"id"`                  // DMR ID
	Network           string        `mapstructure:"network"`             // Network name (for display)
	Address           string        `mapstructure:"address"`             // DMR network server address
	Port              int           `mapstructure:"port"`                // DMR network port
	Password          string        `mapstructure:"password"`            // Network password
	TalkGroup         uint32        `mapstructure:"talk_group"`          // Talk group to bridge
	Slot              uint8         `mapstructure:"slot"`                // DMR slot (1 or 2)
	ColorCode         uint8         `mapstructure:"color_code"`          // Color code
	EnablePrivateCall bool          `mapstructure:"enable_private_call"` // Enable private calls
	RXFreq            uint32        `mapstructure:"rx_freq"`             // RX frequency in Hz
	TXFreq            uint32        `mapstructure:"tx_freq"`             // TX frequency in Hz
	TXPower           uint32        `mapstructure:"tx_power"`            // TX power level
	Latitude          float32       `mapstructure:"latitude"`            // Latitude
	Longitude         float32       `mapstructure:"longitude"`           // Longitude
	Height            int32         `mapstructure:"height"`              // Height above ground
	Location          string        `mapstructure:"location"`            // Location description
	Description       string        `mapstructure:"description"`         // Description
	URL               string        `mapstructure:"url"`                 // URL
	PingInterval      time.Duration `mapstructure:"ping_interval"`       // Keep-alive interval
	AuthTimeout       time.Duration `mapstructure:"auth_timeout"`        // Auth timeout
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

// YSF2DMRConfig holds YSF2DMR bridge configuration
type YSF2DMRConfig struct {
	Enabled bool             `mapstructure:"enabled"`
	YSF     YSF2DMRYSFConfig `mapstructure:"ysf"`
	DMR     YSF2DMRDMRConfig `mapstructure:"dmr"`
	Lookup  DMRLookupConfig  `mapstructure:"lookup"`
	Audio   AudioConfig      `mapstructure:"audio"`
}

// YSF2DMRYSFConfig holds YSF-side configuration for YSF2DMR
type YSF2DMRYSFConfig struct {
	Callsign     string        `mapstructure:"callsign"`
	Suffix       string        `mapstructure:"suffix"`
	LocalAddress string        `mapstructure:"local_address"`
	LocalPort    int           `mapstructure:"local_port"`
	EnableWiresX bool          `mapstructure:"enable_wiresx"`
	HangTime     time.Duration `mapstructure:"hang_time"`
}

// YSF2DMRDMRConfig holds DMR network configuration for YSF2DMR
type YSF2DMRDMRConfig struct {
	Enabled           bool          `mapstructure:"enabled"`
	ID                uint32        `mapstructure:"id"`
	Network           string        `mapstructure:"network"`
	Address           string        `mapstructure:"address"`
	Port              int           `mapstructure:"port"`
	Password          string        `mapstructure:"password"`
	StartupTG         uint32        `mapstructure:"startup_tg"`
	Slot              uint8         `mapstructure:"slot"`
	ColorCode         uint8         `mapstructure:"color_code"`
	EnablePrivateCall bool          `mapstructure:"enable_private_call"`
	RXFreq            uint32        `mapstructure:"rx_freq"`
	TXFreq            uint32        `mapstructure:"tx_freq"`
	TXPower           uint32        `mapstructure:"tx_power"`
	Latitude          float32       `mapstructure:"latitude"`
	Longitude         float32       `mapstructure:"longitude"`
	Height            int32         `mapstructure:"height"`
	Location          string        `mapstructure:"location"`
	Description       string        `mapstructure:"description"`
	URL               string        `mapstructure:"url"`
	PingInterval      time.Duration `mapstructure:"ping_interval"`
	AuthTimeout       time.Duration `mapstructure:"auth_timeout"`
}

// DMRLookupConfig holds DMR ID lookup configuration
type DMRLookupConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	DMRIDFile       string        `mapstructure:"dmr_id_file"`
	AutoDownload    bool          `mapstructure:"auto_download"`
	DownloadURL     string        `mapstructure:"download_url"`
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
}

// AudioConfig holds audio conversion configuration
type AudioConfig struct {
	Gain         float32 `mapstructure:"gain"`
	VOXEnabled   bool    `mapstructure:"vox_enabled"`
	VOXThreshold float32 `mapstructure:"vox_threshold"`
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

	// Populate DMR passwords from environment variables if not set in file
	populateDMRPasswordsFromEnv(&config)

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
	viper.SetDefault("server.talk_max_duration", "3m")
	viper.SetDefault("server.unmute_after", "1m")

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

	// Bridge defaults
	viper.SetDefault("bridges.type", "ysf") // Default to YSF bridge type
	viper.SetDefault("bridges.permanent", false)
	viper.SetDefault("bridges.max_retries", 0)      // 0 = infinite retries
	viper.SetDefault("bridges.retry_delay", "30s")  // Start with 30 second delay
	viper.SetDefault("bridges.health_check", "60s") // Check connection every minute

	// YSF2DMR defaults
	viper.SetDefault("ysf2dmr.enabled", false)
	viper.SetDefault("ysf2dmr.ysf.callsign", "YSF2DMR")
	viper.SetDefault("ysf2dmr.ysf.local_address", "0.0.0.0")
	viper.SetDefault("ysf2dmr.ysf.local_port", 42001)
	viper.SetDefault("ysf2dmr.ysf.enable_wiresx", false)
	viper.SetDefault("ysf2dmr.ysf.hang_time", "5s")
	viper.SetDefault("ysf2dmr.dmr.enabled", true)
	viper.SetDefault("ysf2dmr.dmr.slot", 2)
	viper.SetDefault("ysf2dmr.dmr.color_code", 1)
	viper.SetDefault("ysf2dmr.dmr.startup_tg", 91)
	viper.SetDefault("ysf2dmr.dmr.enable_private_call", false)
	viper.SetDefault("ysf2dmr.dmr.ping_interval", "10s")
	viper.SetDefault("ysf2dmr.dmr.auth_timeout", "30s")
	viper.SetDefault("ysf2dmr.dmr.tx_power", 1)
	viper.SetDefault("ysf2dmr.lookup.enabled", true)
	viper.SetDefault("ysf2dmr.lookup.auto_download", false)
	viper.SetDefault("ysf2dmr.lookup.download_url", "https://radioid.net/static/users.csv")
	viper.SetDefault("ysf2dmr.lookup.refresh_interval", "24h")
	viper.SetDefault("ysf2dmr.audio.gain", 1.0)
	viper.SetDefault("ysf2dmr.audio.vox_enabled", false)
	viper.SetDefault("ysf2dmr.audio.vox_threshold", 0.1)
}

// populateDMRPasswordsFromEnv sets DMR passwords for bridges from environment variables
// Env var pattern: BRIDGE_<BRIDGE_NAME>_DMR_PASSWORD (bridge name uppercased, non-alnum -> _)
// Falls back to global YSF2DMR_DMR_PASSWORD for the ysf2dmr bridge if present.
func populateDMRPasswordsFromEnv(cfg *Config) {
	// Global fallback for ysf2dmr
	if cfg.YSF2DMR.DMR.Password == "" {
		if p := os.Getenv("YSF2DMR_DMR_PASSWORD"); p != "" {
			cfg.YSF2DMR.DMR.Password = p
		}
	}

	for i := range cfg.Bridges {
		b := &cfg.Bridges[i]

		// Only consider DMR bridges with nil/empty password
		if b.DMR == nil {
			continue
		}

		if b.DMR.Password != "" {
			continue
		}

		// Build env var name
		name := b.Name
		// sanitize: replace non-alnum with underscore and uppercase
		sanitized := make([]rune, 0, len(name))
		for _, r := range name {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				if r >= 'a' && r <= 'z' {
					r = r - 'a' + 'A'
				}
				sanitized = append(sanitized, r)
			} else {
				sanitized = append(sanitized, '_')
			}
		}

		envName := "BRIDGE_" + string(sanitized) + "_DMR_PASSWORD"
		if p := os.Getenv(envName); p != "" {
			b.DMR.Password = p
		}
	}
}
