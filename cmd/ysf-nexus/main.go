package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/dbehnke/ysf-nexus/pkg/config"
	"github.com/dbehnke/ysf-nexus/pkg/logger"
	"github.com/dbehnke/ysf-nexus/pkg/reflector"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "ysf-nexus",
		Short: "A modern YSF reflector written in Go",
		Long: `YSF Nexus is a high-performance YSF (Yaesu System Fusion) reflector
with web dashboard, MQTT integration, and bridge capabilities.`,
		Version: fmt.Sprintf("%s (built at %s)", Version, BuildTime),
		RunE:    runServer,
	}

	// Add flags
	rootCmd.Flags().StringP("config", "c", "config.yaml", "Configuration file path")
	rootCmd.Flags().StringP("host", "h", "", "Server host (overrides config)")
	rootCmd.Flags().IntP("port", "p", 0, "Server port (overrides config)")
	rootCmd.Flags().Bool("debug", false, "Enable debug logging (overrides config)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runServer(cmd *cobra.Command, args []string) error {
	// Get command line flags
	configFile, _ := cmd.Flags().GetString("config")
	hostOverride, _ := cmd.Flags().GetString("host")
	portOverride, _ := cmd.Flags().GetInt("port")
	debugOverride, _ := cmd.Flags().GetBool("debug")

	// Load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Apply command line overrides
	if hostOverride != "" {
		cfg.Server.Host = hostOverride
	}
	if portOverride > 0 {
		cfg.Server.Port = portOverride
	}
	if debugOverride {
		cfg.Logging.Level = "debug"
	}

	// Initialize logger
	loggerConfig := logger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		File:        cfg.Logging.File,
		MaxSize:     cfg.Logging.MaxSize,
		MaxBackups:  cfg.Logging.MaxBackups,
		MaxAge:      cfg.Logging.MaxAge,
		Development: cfg.Logging.Level == "debug",
	}

	log, err := logger.New(loggerConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer log.Sync()

	log.Info("YSF Nexus starting",
		logger.String("version", Version),
		logger.String("build_time", BuildTime),
		logger.String("config_file", configFile))

	// Create and start reflector
	r := reflector.New(cfg, log)

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Info("Shutdown signal received", logger.String("signal", sig.String()))
		cancel()
	}()

	// Start the reflector
	if err := r.Start(ctx); err != nil {
		log.Error("Reflector error", logger.Error(err))
		return err
	}

	return nil
}