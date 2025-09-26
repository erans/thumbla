package fetchers

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestLocalFetcher(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "thumbla_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testContent := "test image content"
	testFile := filepath.Join(tempDir, "test.jpg")
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create fetcher config
	config := map[string]interface{}{
		"name": "testLocal",
		"type": "local",
		"path": tempDir,
	}

	fetcher := NewLocalFetcher(config)

	tests := []struct {
		name         string
		url          string
		expectError  bool
		expectedType string
	}{
		{
			name:         "fetch existing file",
			url:          "test.jpg",
			expectError:  false,
			expectedType: "image/jpeg",
		},
		{
			name:        "fetch non-existent file",
			url:         "nonexistent.jpg",
			expectError: true,
		},
		{
			name:        "fetch with path traversal attempt",
			url:         "../../../etc/passwd",
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

			// Read and verify content
			content, err := io.ReadAll(body)
			if err != nil {
				t.Errorf("Failed to read body: %v", err)
				return
			}

			if string(content) != testContent {
				t.Errorf("Expected content %s, got %s", testContent, string(content))
			}
		})
	}
}

func TestLocalFetcher_GetName(t *testing.T) {
	config := map[string]interface{}{
		"name": "testFetcher",
		"type": "local",
		"path": "/test/path",
	}

	fetcher := NewLocalFetcher(config)
	if fetcher.GetName() != "testFetcher" {
		t.Errorf("Expected name 'testFetcher', got %s", fetcher.GetName())
	}
}

func TestLocalFetcher_GetFetcherType(t *testing.T) {
	config := map[string]interface{}{
		"name": "testFetcher",
		"type": "local",
		"path": "/test/path",
	}

	fetcher := NewLocalFetcher(config)
	if fetcher.GetFetcherType() != "local" {
		t.Errorf("Expected type 'local', got %s", fetcher.GetFetcherType())
	}
}