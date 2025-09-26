package fetchers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/erans/thumbla/config"
	"github.com/gofiber/fiber/v2"
)

func TestHTTPFetcher(t *testing.T) {
	// Set up test config for HTTP timeout
	cfg := &config.Config{
		Server: config.ServerConfig{
			HTTPTimeout: 30,
		},
	}
	config.SetConfig(cfg)

	// Create a test HTTP server
	testContent := "test image content"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/test.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testContent))
		case "/test.png":
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testContent))
		case "/not-found":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	// Create fetcher config
	config := map[string]interface{}{
		"name":          "testHTTP",
		"type":          "http",
		"restrictHosts": []interface{}{server.URL[7:]}, // Remove http:// prefix
	}

	fetcher := NewHTTPFetcher(config)

	tests := []struct {
		name         string
		url          string
		expectError  bool
		expectedType string
	}{
		{
			name:         "fetch JPEG image",
			url:          server.URL + "/test.jpg",
			expectError:  false,
			expectedType: "image/jpeg",
		},
		{
			name:         "fetch PNG image",
			url:          server.URL + "/test.png",
			expectError:  false,
			expectedType: "image/png",
		},
		{
			name:        "fetch non-existent resource",
			url:         server.URL + "/not-found",
			expectError: true,
		},
		{
			name:        "fetch from restricted host",
			url:         "http://evil.com/test.jpg",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a nil context for testing (fetchers don't use context for file operations)
			var ctx *fiber.Ctx

			body, contentType, err := fetcher.Fetch(ctx, tt.url)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if contentType != tt.expectedType {
				t.Errorf("Expected content type %s, got %s", tt.expectedType, contentType)
			}

			if body == nil {
				t.Error("Expected body but got nil")
			}
		})
	}
}

func TestHTTPFetcher_RestrictPaths(t *testing.T) {
	// Set up test config for HTTP timeout
	cfg := &config.Config{
		Server: config.ServerConfig{
			HTTPTimeout: 30,
		},
	}
	config.SetConfig(cfg)

	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test content"))
	}))
	defer server.Close()

	// Create fetcher config with path restrictions
	config := map[string]interface{}{
		"name":          "testHTTP",
		"type":          "http",
		"restrictPaths": []interface{}{"/allowed/"},
	}

	fetcher := NewHTTPFetcher(config)

	tests := []struct {
		name        string
		url         string
		expectError bool
	}{
		{
			name:        "allowed path",
			url:         server.URL + "/allowed/test.jpg",
			expectError: false,
		},
		{
			name:        "restricted path",
			url:         server.URL + "/forbidden/test.jpg",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a nil context for testing (fetchers don't use context for file operations)
			var ctx *fiber.Ctx

			_, _, err := fetcher.Fetch(ctx, tt.url)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}