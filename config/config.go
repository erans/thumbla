package config

import (
	"os"
	"strconv"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

var (
	globalConfig *Config
)

// PathConfig represents configuration for a path serving images
type PathConfig struct {
	Path         string `yaml:"path"`
	FetcherName  string `yaml:"fetcherName"`
	CacheControl string `yaml:"cacheControl"`
}

// ServerConfig provides server-level configuration options
type ServerConfig struct {
	MaxRequestSize     int64 `yaml:"maxRequestSize"`     // In bytes, default 100MB
	ReadTimeout        int   `yaml:"readTimeout"`        // In seconds, default 30
	WriteTimeout       int   `yaml:"writeTimeout"`       // In seconds, default 30
	HTTPTimeout        int   `yaml:"httpTimeout"`        // In seconds for HTTP fetcher, default 30
	MaxImageDimension  int   `yaml:"maxImageDimension"`  // Max image width or height in pixels, default 10000
	MaxImageSizeBytes  int64 `yaml:"maxImageSizeBytes"`  // Max image file size in bytes, default 50MB
	RateLimit          RateLimitConfig `yaml:"rateLimit"`
}

// RateLimitConfig provides rate limiting configuration
type RateLimitConfig struct {
	Enabled     bool `yaml:"enabled"`     // Enable/disable rate limiting, default true
	MaxRequests int  `yaml:"maxRequests"` // Max requests per window, default 1000
	WindowSec   int  `yaml:"windowSec"`   // Time window in seconds, default 60
}

// Config provides a configuration struct for the server
type Config struct {
	CacheControlHeader string                   `yaml:"cacheControlHeader"`
	DebugLevel         string                   `yaml:"debugLevel"`
	Fetchers           []map[string]interface{} `yaml:"fetchers"`
	Paths              []PathConfig             `yaml:"paths"`
	Server             ServerConfig             `yaml:"server"`
	FaceAPI            struct {
		DefaultProvider  string `yaml:"defaultProvider"`
		MicrosoftFaceAPI struct {
			Key string `yaml:"key"`
			URL string `yaml:"url"`
		} `yaml:"microsoftFaceAPI"`
		GoogleCloudVisionAPI struct {
			Key string `yaml:"key"`
		} `yaml:"googleCloudVisionAPI"`
		AWSRekognition struct {
			Region string `yaml:"region"`
		} `yaml:"awsRekognition"`
	} `yaml:"faceapi"`
	Cache struct {
		Active   bool   `yaml:"active"`
		Provider string `yaml:"provider"`
		InMemory struct {
			Size int `yaml:"size"`
		} `yaml:"inmemory"`
		Redis struct {
			Address    string `yaml:"host"`
			Password   string `yaml:"string"`
			DB         int    `yaml:"db"`
			MaxLRUSize int    `yaml:"maxLRUSize"`
		} `yaml:"redis"`
	} `yaml:"cache"`
}

// GetFetcherConfigKeyValue returns a configuration key value of a fetcher
func (cfg *Config) GetFetcherConfigKeyValue(fetcherType string, key string) interface{} {
	for _, v := range cfg.Fetchers {
		if typeValue, ok := v["type"]; ok {
			if typeValue == fetcherType {
				if v, ok := v[key]; ok {
					return v
				}
			}
		}
	}

	return ""
}

// GetPathConfigByPath returns the path config by the specified path
func (cfg *Config) GetPathConfigByPath(path string) *PathConfig {
	for _, p := range cfg.Paths {
		// First try exact match
		if p.Path == path {
			return &p
		}
		// Then try prefix match if the config path ends with '/'
		if strings.HasSuffix(p.Path, "/") && strings.HasPrefix(path, p.Path) {
			return &p
		}
	}

	return nil
}

// LoadConfig loads the config file
func LoadConfig(configFile string) (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	// Expand envirovment variables defined in the config
	data = []byte(os.ExpandEnv(string(data)))

	var c Config
	if err := yaml.Unmarshal([]byte(data), &c); err != nil {
		return nil, err
	}

	// Override with environment variables if set
	applyEnvironmentOverrides(&c)

	return &c, nil
}

// applyEnvironmentOverrides applies environment variable overrides to config
func applyEnvironmentOverrides(cfg *Config) {
	// Max request size override
	if maxSizeStr := os.Getenv("THUMBLA_MAX_REQUEST_SIZE"); maxSizeStr != "" {
		if maxSize, err := strconv.ParseInt(maxSizeStr, 10, 64); err == nil {
			cfg.Server.MaxRequestSize = maxSize
		}
	}

	// Read timeout override
	if readTimeoutStr := os.Getenv("THUMBLA_READ_TIMEOUT"); readTimeoutStr != "" {
		if readTimeout, err := strconv.Atoi(readTimeoutStr); err == nil {
			cfg.Server.ReadTimeout = readTimeout
		}
	}

	// Write timeout override
	if writeTimeoutStr := os.Getenv("THUMBLA_WRITE_TIMEOUT"); writeTimeoutStr != "" {
		if writeTimeout, err := strconv.Atoi(writeTimeoutStr); err == nil {
			cfg.Server.WriteTimeout = writeTimeout
		}
	}

	// HTTP timeout override
	if httpTimeoutStr := os.Getenv("THUMBLA_HTTP_TIMEOUT"); httpTimeoutStr != "" {
		if httpTimeout, err := strconv.Atoi(httpTimeoutStr); err == nil {
			cfg.Server.HTTPTimeout = httpTimeout
		}
	}

	// Rate limit enabled override
	if rateLimitEnabledStr := os.Getenv("THUMBLA_RATE_LIMIT_ENABLED"); rateLimitEnabledStr != "" {
		if rateLimitEnabled, err := strconv.ParseBool(rateLimitEnabledStr); err == nil {
			cfg.Server.RateLimit.Enabled = rateLimitEnabled
		}
	}

	// Rate limit max requests override
	if rateLimitMaxStr := os.Getenv("THUMBLA_RATE_LIMIT_MAX"); rateLimitMaxStr != "" {
		if rateLimitMax, err := strconv.Atoi(rateLimitMaxStr); err == nil {
			cfg.Server.RateLimit.MaxRequests = rateLimitMax
		}
	}

	// Rate limit window override
	if rateLimitWindowStr := os.Getenv("THUMBLA_RATE_LIMIT_WINDOW"); rateLimitWindowStr != "" {
		if rateLimitWindow, err := strconv.Atoi(rateLimitWindowStr); err == nil {
			cfg.Server.RateLimit.WindowSec = rateLimitWindow
		}
	}

	// Max image dimension override
	if maxDimensionStr := os.Getenv("THUMBLA_MAX_IMAGE_DIMENSION"); maxDimensionStr != "" {
		if maxDimension, err := strconv.Atoi(maxDimensionStr); err == nil {
			cfg.Server.MaxImageDimension = maxDimension
		}
	}

	// Max image size override
	if maxImageSizeStr := os.Getenv("THUMBLA_MAX_IMAGE_SIZE"); maxImageSizeStr != "" {
		if maxImageSize, err := strconv.ParseInt(maxImageSizeStr, 10, 64); err == nil {
			cfg.Server.MaxImageSizeBytes = maxImageSize
		}
	}
}

// SetConfig set currently active config
func SetConfig(cfg *Config) {
	globalConfig = cfg
}

// GetConfig return the currently active global config
func GetConfig() *Config {
	return globalConfig
}

// GetMaxRequestSize returns the max request size with default fallback
func (cfg *Config) GetMaxRequestSize() int64 {
	if cfg.Server.MaxRequestSize <= 0 {
		return 100 * 1024 * 1024 // Default 100MB
	}
	return cfg.Server.MaxRequestSize
}

// GetReadTimeout returns the read timeout with default fallback
func (cfg *Config) GetReadTimeout() int {
	if cfg.Server.ReadTimeout <= 0 {
		return 30 // Default 30 seconds
	}
	return cfg.Server.ReadTimeout
}

// GetWriteTimeout returns the write timeout with default fallback
func (cfg *Config) GetWriteTimeout() int {
	if cfg.Server.WriteTimeout <= 0 {
		return 30 // Default 30 seconds
	}
	return cfg.Server.WriteTimeout
}

// GetHTTPTimeout returns the HTTP client timeout with default fallback
func (cfg *Config) GetHTTPTimeout() int {
	if cfg.Server.HTTPTimeout <= 0 {
		return 30 // Default 30 seconds
	}
	return cfg.Server.HTTPTimeout
}

// IsRateLimitEnabled returns whether rate limiting is enabled with default fallback
func (cfg *Config) IsRateLimitEnabled() bool {
	// Default to enabled if not explicitly set
	return cfg.Server.RateLimit.Enabled
}

// GetRateLimitMaxRequests returns the max requests for rate limiting with default fallback
func (cfg *Config) GetRateLimitMaxRequests() int {
	if cfg.Server.RateLimit.MaxRequests <= 0 {
		return 1000 // Default 1000 requests
	}
	return cfg.Server.RateLimit.MaxRequests
}

// GetRateLimitWindow returns the rate limit window in seconds with default fallback
func (cfg *Config) GetRateLimitWindow() int {
	if cfg.Server.RateLimit.WindowSec <= 0 {
		return 60 // Default 60 seconds
	}
	return cfg.Server.RateLimit.WindowSec
}

// GetMaxImageDimension returns the max image dimension with default fallback
func (cfg *Config) GetMaxImageDimension() int {
	if cfg.Server.MaxImageDimension <= 0 {
		return 10000 // Default 10000 pixels
	}
	return cfg.Server.MaxImageDimension
}

// GetMaxImageSizeBytes returns the max image file size with default fallback
func (cfg *Config) GetMaxImageSizeBytes() int64 {
	if cfg.Server.MaxImageSizeBytes <= 0 {
		return 50 * 1024 * 1024 // Default 50MB
	}
	return cfg.Server.MaxImageSizeBytes
}
