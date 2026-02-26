package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var version = "dev" // set via ldflags during build

func main() {
	// Command-line flags
	configFile := flag.String("config", "", "Path to configuration file (JSON)")
	port := flag.Int("port", 0, "Port to listen on (overrides config)")
	host := flag.String("host", "", "Host to bind to (overrides config)")
	rootDir := flag.String("dir", "", "Root directory to serve (overrides config)")
	enableListing := flag.Bool("list", false, "Enable directory listing")
	generateConfig := flag.String("generate-config", "", "Generate example config file and exit")
	showVersion := flag.Bool("version", false, "Show version and exit")
	showHelp := flag.Bool("help", false, "Show help and exit")

	flag.Parse()

	// Show version
	if *showVersion {
		fmt.Printf("koryx-serv version %s\n", version)
		os.Exit(0)
	}

	// Show help
	if *showHelp {
		printHelp()
		os.Exit(0)
	}

	// Generate example configuration file
	if *generateConfig != "" {
		if err := SaveConfig(*generateConfig, DefaultConfig()); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Example configuration saved to: %s\n", *generateConfig)
		os.Exit(0)
	}

	// Load configuration
	config, err := loadConfiguration(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Override with command-line flags
	if *port > 0 {
		config.Server.Port = *port
	}
	if *host != "" {
		config.Server.Host = *host
	}
	if *rootDir != "" {
		config.Server.RootDir = *rootDir
	}
	if *enableListing {
		config.Features.DirectoryListing = true
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration: %v\n", err)
		os.Exit(1)
	}

	// Create logger
	logger, err := NewLogger(&config.Logging)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating logger: %v\n", err)
		os.Exit(1)
	}

	// Create and start server
	server := NewServer(config, logger)

	// Configure SIGINT/SIGTERM handler
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or server error
	select {
	case err := <-errChan:
		logger.Error("Server error: %v", err)
		os.Exit(1)
	case sig := <-sigChan:
		logger.Info("\nReceived signal %v, shutting down gracefully...", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logger.Error("Graceful shutdown failed: %v", err)
			os.Exit(1)
		}
		logger.Info("Server stopped gracefully")
	}
}

// loadConfiguration loads the configuration
func loadConfiguration(configFile string) (*Config, error) {
	if configFile == "" {
		return DefaultConfig(), nil
	}

	config, err := LoadConfig(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config file: %w", err)
	}

	return config, nil
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	// Validate port
	if config.Server.Port < 1 || config.Server.Port > 65535 {
		return fmt.Errorf("invalid port: %d (must be between 1-65535)", config.Server.Port)
	}

	// Validate root directory
	if info, err := os.Stat(config.Server.RootDir); err != nil {
		return fmt.Errorf("root directory error: %w", err)
	} else if !info.IsDir() {
		return fmt.Errorf("root path is not a directory: %s", config.Server.RootDir)
	}

	// Validate HTTPS settings
	if config.Security.EnableHTTPS {
		if config.Security.CertFile == "" || config.Security.KeyFile == "" {
			return fmt.Errorf("HTTPS enabled but cert_file or key_file not specified")
		}
		if _, err := os.Stat(config.Security.CertFile); err != nil {
			return fmt.Errorf("certificate file not found: %s", config.Security.CertFile)
		}
		if _, err := os.Stat(config.Security.KeyFile); err != nil {
			return fmt.Errorf("key file not found: %s", config.Security.KeyFile)
		}
	}

	// Validate basic authentication
	if config.Security.BasicAuth != nil && config.Security.BasicAuth.Enabled {
		if config.Security.BasicAuth.Username == "" || config.Security.BasicAuth.Password == "" {
			return fmt.Errorf("basic auth enabled but username or password not specified")
		}
		if config.Security.BasicAuth.Realm == "" {
			config.Security.BasicAuth.Realm = "Restricted"
		}
	}

	// Validate compression level
	if config.Performance.CompressionLevel < 1 || config.Performance.CompressionLevel > 9 {
		config.Performance.CompressionLevel = 6
	}

	// Validate log level
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[config.Logging.Level] {
		config.Logging.Level = "info"
	}

	return nil
}

// printHelp prints the help message
func printHelp() {
	fmt.Printf(`koryx-serv - Simple HTTP file server with advanced features

VERSION:
  %s

USAGE:
  koryx-serv [options]

OPTIONS:
  -config string
        Path to configuration file (JSON)

  -port int
        Port to listen on (overrides config)

  -host string
        Host to bind to (overrides config)

  -dir string
        Root directory to serve (overrides config)

  -list
        Enable directory listing

  -generate-config string
        Generate example config file and exit

  -version
        Show version and exit

  -help
        Show this help message

EXAMPLES:
  # Serve current directory on port 8080
  koryx-serv

  # Serve specific directory on custom port
  koryx-serv -dir /var/www -port 3000

  # Enable directory listing
  koryx-serv -list

  # Use configuration file
  koryx-serv -config config.json

  # Generate example configuration
  koryx-serv -generate-config config.example.json

CONFIGURATION:
  Configuration can be provided via a JSON file using the -config flag.
  Use -generate-config to create an example configuration file.

FEATURES:
  • Static file serving
  • Directory listing (optional)
  • HTTPS/TLS support
  • Basic authentication
  • CORS support
  • Rate limiting
  • IP whitelist/blacklist
  • Gzip compression
  • Cache headers
  • ETags
  • SPA mode
  • Custom error pages
  • Access logging
  • Security headers

For more information, visit: https://github.com/koryxio/koryx-serv
`, version)
}
