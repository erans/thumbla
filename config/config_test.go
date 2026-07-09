package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tempDir, err := os.MkdirTemp("", "thumbla_config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configContent := `
debugLevel: debug
cacheControlHeader: "public, max-age=3600"

fetchers:
  - name: testLocal
    type: local
    path: /tmp/images

  - name: testHTTP
    type: http
    secure: true
    restrictHosts:
      - example.com

paths:
  - path: "/images/"
    fetcherName: testLocal
    cacheControl: "public, max-age=7200"

  - path: "/external/"
    fetcherName: testHTTP

faceapi:
  defaultProvider: "aws"
  awsRekognition:
    region: "us-east-1"

cache:
  active: true
  provider: "inmemory"
  inmemory:
    size: 1000
`

	configFile := filepath.Join(tempDir, "test-config.yml")
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Test loading the config
	cfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test basic properties
	if cfg.DebugLevel != "debug" {
		t.Errorf("Expected debugLevel 'debug', got '%s'", cfg.DebugLevel)
	}

	if cfg.CacheControlHeader != "public, max-age=3600" {
		t.Errorf("Expected cacheControlHeader 'public, max-age=3600', got '%s'", cfg.CacheControlHeader)
	}

	// Test fetchers
	if len(cfg.Fetchers) != 2 {
		t.Errorf("Expected 2 fetchers, got %d", len(cfg.Fetchers))
	}

	// Test paths
	if len(cfg.Paths) != 2 {
		t.Errorf("Expected 2 paths, got %d", len(cfg.Paths))
	}

	if cfg.Paths[0].Path != "/images/" {
		t.Errorf("Expected first path '/images/', got '%s'", cfg.Paths[0].Path)
	}

	if cfg.Paths[0].FetcherName != "testLocal" {
		t.Errorf("Expected first path fetcherName 'testLocal', got '%s'", cfg.Paths[0].FetcherName)
	}

	// Test face API config
	if cfg.FaceAPI.DefaultProvider != "aws" {
		t.Errorf("Expected defaultProvider 'aws', got '%s'", cfg.FaceAPI.DefaultProvider)
	}

	// Test cache config
	if !cfg.Cache.Active {
		t.Error("Expected cache to be active")
	}

	if cfg.Cache.Provider != "inmemory" {
		t.Errorf("Expected cache provider 'inmemory', got '%s'", cfg.Cache.Provider)
	}
}

func TestLoadConfigRedisPasswordAndTTL(t *testing.T) {
	// Create a temporary config file
	tempDir, err := os.MkdirTemp("", "thumbla_config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configContent := `
cache:
  provider: "redis"
  redis:
    host: "localhost:6379"
    password: "secret"
    ttl: 42
`

	configFile := filepath.Join(tempDir, "redis-config.yml")
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Cache.Redis.Password != "secret" {
		t.Fatalf("Expected redis password 'secret', got '%s'", cfg.Cache.Redis.Password)
	}
	if cfg.GetRedisTTL() != 42 {
		t.Fatalf("Expected redis TTL 42, got %d", cfg.GetRedisTTL())
	}
}

func TestGetRedisTTLDefault(t *testing.T) {
	cfg := &Config{}
	if got := cfg.GetRedisTTL(); got != 3600 {
		t.Fatalf("Expected default redis TTL 3600, got %d", got)
	}
}

func TestLoadConfig_RedisPassword(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "thumbla_redis_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configContent := `
cache:
  active: true
  provider: "redis"
  redis:
    host: "localhost:6379"
    password: "s3cret"
    ttl: 120
`
	configFile := filepath.Join(tempDir, "test-config.yml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Cache.Redis.Password != "s3cret" {
		t.Errorf("expected password 's3cret', got %q", cfg.Cache.Redis.Password)
	}
	if cfg.GetRedisTTL() != 120 {
		t.Errorf("expected TTL 120, got %d", cfg.GetRedisTTL())
	}
}

func TestLoadConfig_TrustedProxies(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "thumbla_proxy_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configContent := `
server:
  trustedProxies:
    enabled: true
    proxies:
      - "10.0.0.0/8"
      - "192.168.0.0/16"
    proxyHeader: "X-Forwarded-For"
`
	configFile := filepath.Join(tempDir, "test-config.yml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.IsTrustedProxiesEnabled() {
		t.Fatal("expected trusted proxies to be enabled")
	}
	if cfg.Server.TrustedProxies.ProxyHeader != "X-Forwarded-For" {
		t.Errorf("expected proxy header X-Forwarded-For, got %q", cfg.Server.TrustedProxies.ProxyHeader)
	}
	if len(cfg.Server.TrustedProxies.Proxies) != 2 {
		t.Fatalf("expected two trusted proxy entries, got %d", len(cfg.Server.TrustedProxies.Proxies))
	}
}

func TestLoadConfig_InvalidFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.yml")
	if err == nil {
		t.Error("Expected error for non-existent config file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	// Create a temporary invalid YAML file
	tempDir, err := os.MkdirTemp("", "thumbla_config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	invalidYAML := `
debugLevel: debug
fetchers:
  - name: test
    type: local
    invalid yaml structure {{{
`

	configFile := filepath.Join(tempDir, "invalid-config.yml")
	err = os.WriteFile(configFile, []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err = LoadConfig(configFile)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestGetPathConfigByPath(t *testing.T) {
	cfg := &Config{
		Paths: []PathConfig{
			{Path: "/images/", FetcherName: "local"},
			{Path: "/external/", FetcherName: "http"},
		},
	}

	SetConfig(cfg)

	tests := []struct {
		name         string
		requestPath  string
		expectedName string
		shouldFind   bool
	}{
		{
			name:         "exact match",
			requestPath:  "/images/",
			expectedName: "local",
			shouldFind:   true,
		},
		{
			name:         "prefix match",
			requestPath:  "/images/test.jpg",
			expectedName: "local",
			shouldFind:   true,
		},
		{
			name:         "no match",
			requestPath:  "/api/test",
			expectedName: "",
			shouldFind:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pathConfig := cfg.GetPathConfigByPath(tt.requestPath)

			if tt.shouldFind {
				if pathConfig == nil {
					t.Error("Expected to find path config but got nil")
					return
				}
				if pathConfig.FetcherName != tt.expectedName {
					t.Errorf("Expected fetcher name '%s', got '%s'", tt.expectedName, pathConfig.FetcherName)
				}
			} else {
				if pathConfig != nil {
					t.Error("Expected nil path config but got a result")
				}
			}
		})
	}
}
