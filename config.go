package main

import (
	"encoding/json"
	"os"
	"time"
)

// Config represents the full server configuration
type Config struct {
	Server        ServerConfig        `json:"server"`
	Security      SecurityConfig      `json:"security"`
	Performance   PerformanceConfig   `json:"performance"`
	Logging       LoggingConfig       `json:"logging"`
	Features      FeaturesConfig      `json:"features"`
	RuntimeConfig *RuntimeConfigConfig `json:"runtime_config,omitempty"`
}

// ServerConfig contains basic server settings
type ServerConfig struct {
	Port         int    `json:"port"`
	Host         string `json:"host"`
	RootDir      string `json:"root_dir"`
	ReadTimeout  int    `json:"read_timeout"`   // seconds
	WriteTimeout int    `json:"write_timeout"`  // seconds
}

// SecurityConfig contains security settings
type SecurityConfig struct {
	EnableHTTPS      bool              `json:"enable_https"`
	CertFile         string            `json:"cert_file"`
	KeyFile          string            `json:"key_file"`
	BasicAuth        *BasicAuthConfig  `json:"basic_auth,omitempty"`
	CORS             *CORSConfig       `json:"cors,omitempty"`
	RateLimit        *RateLimitConfig  `json:"rate_limit,omitempty"`
	IPWhitelist      []string          `json:"ip_whitelist,omitempty"`
	IPBlacklist      []string          `json:"ip_blacklist,omitempty"`
	BlockHiddenFiles bool              `json:"block_hidden_files"`
	AllowedPaths     []string          `json:"allowed_paths,omitempty"`
	BlockedPaths     []string          `json:"blocked_paths,omitempty"`
}

// BasicAuthConfig configures HTTP basic authentication
type BasicAuthConfig struct {
	Enabled  bool   `json:"enabled"`
	Username string `json:"username"`
	Password string `json:"password"`
	Realm    string `json:"realm"`
}

// CORSConfig contains CORS settings
type CORSConfig struct {
	Enabled          bool     `json:"enabled"`
	AllowedOrigins   []string `json:"allowed_origins"`
	AllowedMethods   []string `json:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers"`
	AllowCredentials bool     `json:"allow_credentials"`
	MaxAge           int      `json:"max_age"`
}

// RateLimitConfig defines rate limit settings
type RateLimitConfig struct {
	Enabled       bool `json:"enabled"`
	RequestsPerIP int  `json:"requests_per_ip"` // requests per minute
	BurstSize     int  `json:"burst_size"`
}

// PerformanceConfig contains performance settings
type PerformanceConfig struct {
	EnableCompression bool              `json:"enable_compression"`
	CompressionLevel  int               `json:"compression_level"` // 1-9
	EnableCache       bool              `json:"enable_cache"`
	CacheMaxAge       int               `json:"cache_max_age"` // seconds
	EnableETags       bool              `json:"enable_etags"`
	CustomHeaders     map[string]string `json:"custom_headers,omitempty"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Enabled     bool   `json:"enabled"`
	Level       string `json:"level"` // debug, info, warn, error
	AccessLog   bool   `json:"access_log"`
	ErrorLog    bool   `json:"error_log"`
	LogFile     string `json:"log_file,omitempty"`
	ColorOutput bool   `json:"color_output"`
}

// FeaturesConfig contains additional features
type FeaturesConfig struct {
	DirectoryListing bool     `json:"directory_listing"`
	IndexFiles       []string `json:"index_files"`
	SPAMode          bool     `json:"spa_mode"` // redirect all routes to index.html
	SPAIndex         string   `json:"spa_index"`
	CustomErrorPages map[string]string `json:"custom_error_pages,omitempty"`
}

// RuntimeConfigConfig configures runtime config output
type RuntimeConfigConfig struct {
	Enabled      bool     `json:"enabled"`
	Route        string   `json:"route"`          // route where config is served (default: /runtime-config.js)
	Format       string   `json:"format"`         // "js" or "json" (default: js)
	VarName      string   `json:"var_name"`       // JavaScript variable name (default: APP_CONFIG)
	EnvPrefix    string   `json:"env_prefix"`     // env var prefix (e.g., "APP_" or "RUNTIME_")
	EnvVariables []string `json:"env_variables"`  // specific variable list (alternative to prefix)
	NoCache      bool     `json:"no_cache"`       // if true, add no-cache headers
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         8080,
			Host:         "0.0.0.0",
			RootDir:      ".",
			ReadTimeout:  30,
			WriteTimeout: 30,
		},
		Security: SecurityConfig{
			EnableHTTPS:      false,
			BlockHiddenFiles: true,
		},
		Performance: PerformanceConfig{
			EnableCompression: true,
			CompressionLevel:  6,
			EnableCache:       true,
			CacheMaxAge:       3600,
			EnableETags:       true,
		},
		Logging: LoggingConfig{
			Enabled:     true,
			Level:       "info",
			AccessLog:   true,
			ErrorLog:    true,
			ColorOutput: true,
		},
		Features: FeaturesConfig{
			DirectoryListing: false,
			IndexFiles:       []string{"index.html", "index.htm"},
			SPAMode:          false,
			SPAIndex:         "index.html",
		},
	}
}

// LoadConfig loads configuration from a JSON file
func LoadConfig(filename string) (*Config, error) {
	config := DefaultConfig()

	// If file does not exist, return default configuration
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return config, nil
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(config); err != nil {
		return nil, err
	}

	return config, nil
}

// SaveConfig saves configuration to a JSON file
func SaveConfig(filename string, config *Config) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(config)
}

// GetReadTimeout returns the read timeout as Duration
func (c *ServerConfig) GetReadTimeout() time.Duration {
	return time.Duration(c.ReadTimeout) * time.Second
}

// GetWriteTimeout returns the write timeout as Duration
func (c *ServerConfig) GetWriteTimeout() time.Duration {
	return time.Duration(c.WriteTimeout) * time.Second
}
