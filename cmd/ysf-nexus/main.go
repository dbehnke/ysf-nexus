package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
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
	rootCmd.Flags().StringP("host", "h", "0.0.0.0", "Server host")
	rootCmd.Flags().IntP("port", "p", 42000, "Server port")
	rootCmd.Flags().Bool("debug", false, "Enable debug logging")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func runServer(cmd *cobra.Command, args []string) error {
	// Get configuration
	configFile, _ := cmd.Flags().GetString("config")
	host, _ := cmd.Flags().GetString("host")
	port, _ := cmd.Flags().GetInt("port")
	debug, _ := cmd.Flags().GetBool("debug")

	fmt.Printf("YSF Nexus %s starting...\n", Version)
	fmt.Printf("Config: %s\n", configFile)
	fmt.Printf("Listening on: %s:%d\n", host, port)

	if debug {
		fmt.Println("Debug logging enabled")
	}

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutdown signal received, stopping server...")
		cancel()
	}()

	// TODO: Initialize and start the actual server components
	fmt.Println("Server starting... (placeholder)")

	// Wait for shutdown
	<-ctx.Done()
	fmt.Println("Server stopped")
	return nil
}