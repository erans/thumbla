package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erans/thumbla/config"
	"github.com/erans/thumbla/fetchers"
	"github.com/erans/thumbla/handlers"
	"github.com/erans/thumbla/manipulators"
	"github.com/erans/thumbla/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func createTestApp(tempDir string) *fiber.App {
	// Create test configuration
	cfg := &config.Config{
		DebugLevel:         "debug",
		CacheControlHeader: "public, max-age=3600",
		Fetchers: []map[string]interface{}{
			{
				"name": "local",
				"type": "local",
				"path": tempDir,
			},
		},
		Paths: []config.PathConfig{
			{
				Path:         "/i/local",
				FetcherName:  "local",
				CacheControl: "public, max-age=7200",
			},
		},
		Server: config.ServerConfig{
			MaxRequestSize:    100 * 1024 * 1024, // 100MB
			ReadTimeout:       30,
			WriteTimeout:      30,
			HTTPTimeout:       30,
			MaxImageDimension: 10000,              // 10000x10000 pixels
			MaxImageSizeBytes: 50 * 1024 * 1024,  // 50MB
			RateLimit: config.RateLimitConfig{
				Enabled:     true,
				MaxRequests: 1000,
				WindowSec:   60,
			},
		},
	}

	// Set global config
	config.SetConfig(cfg)

	// Initialize logging
	middleware.InitGlobalLogger(cfg.DebugLevel, true)

	// Initialize components
	fetchers.InitFetchers(cfg)
	manipulators.InitManipulators(cfg)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		BodyLimit:    int(cfg.GetMaxRequestSize()),
		ReadTimeout:  time.Duration(cfg.GetReadTimeout()) * time.Second,
		WriteTimeout: time.Duration(cfg.GetWriteTimeout()) * time.Second,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			logger := middleware.GetLoggerFromContext(c)
			logger.Error().Err(err).Msg("Request error")
			return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
		},
	})

	// Add middleware
	app.Use(middleware.New())
	app.Use(recover.New())

	// Add routes
	app.Get("/health", handlers.HandleHealth)

	// Add dynamic routes based on config paths like main app
	for _, p := range cfg.Paths {
		var path = p.Path
		if strings.Index(path, ":url") == -1 {
			path = fmt.Sprintf("%s/:url/*", path)
		}
		app.Get(path, handlers.HandleImage)
	}

	return app
}

func createTestImage(width, height int, format string) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Create a simple pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if (x+y)%20 < 10 {
				img.Set(x, y, color.RGBA{255, 0, 0, 255}) // Red
			} else {
				img.Set(x, y, color.RGBA{0, 255, 0, 255}) // Green
			}
		}
	}

	var buf bytes.Buffer
	var err error

	switch format {
	case "jpeg", "jpg":
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	case "png":
		err = png.Encode(&buf, img)
	default:
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	}

	return buf.Bytes(), err
}

func TestIntegration_HealthCheck(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "thumbla_integration_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	app := createTestApp(tempDir)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req, 5*1000) // 5 second timeout
	if err != nil {
		t.Fatalf("Failed to perform request: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status %d, got %d", fiber.StatusOK, resp.StatusCode)
	}
}

func TestIntegration_ImageProcessing(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "thumbla_integration_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test images
	jpegData, err := createTestImage(200, 200, "jpeg")
	if err != nil {
		t.Fatalf("Failed to create test JPEG: %v", err)
	}

	pngData, err := createTestImage(150, 150, "png")
	if err != nil {
		t.Fatalf("Failed to create test PNG: %v", err)
	}

	// Write test files
	err = os.WriteFile(filepath.Join(tempDir, "test.jpg"), jpegData, 0644)
	if err != nil {
		t.Fatalf("Failed to write test JPEG: %v", err)
	}

	err = os.WriteFile(filepath.Join(tempDir, "test.png"), pngData, 0644)
	if err != nil {
		t.Fatalf("Failed to write test PNG: %v", err)
	}

	app := createTestApp(tempDir)

	tests := []struct {
		name           string
		url            string
		expectedStatus int
		expectedType   string
	}{
		{
			name:           "simple image fetch",
			url:            "/i/local/test.jpg/output:f=jpg",
			expectedStatus: fiber.StatusOK,
			expectedType:   "image/jpeg",
		},
		{
			name:           "resize image",
			url:            "/i/local/test.jpg/resize:w=100/output:f=jpg",
			expectedStatus: fiber.StatusOK,
			expectedType:   "image/jpeg",
		},
		{
			name:           "format conversion",
			url:            "/i/local/test.jpg/output:f=png",
			expectedStatus: fiber.StatusOK,
			expectedType:   "image/png",
		},
		{
			name:           "multiple operations",
			url:            "/i/local/test.png/resize:w=75/rotate:a=90/output:f=jpg",
			expectedStatus: fiber.StatusOK,
			expectedType:   "image/jpeg",
		},
		{
			name:           "crop and resize",
			url:            "/i/local/test.jpg/crop:x=25,y=25,w=100,h=100/resize:w=50/output:f=png",
			expectedStatus: fiber.StatusOK,
			expectedType:   "image/png",
		},
		{
			name:           "quality setting",
			url:            "/i/local/test.jpg/resize:w=80/output:f=jpg,q=50",
			expectedStatus: fiber.StatusOK,
			expectedType:   "image/jpeg",
		},
		{
			name:           "non-existent file",
			url:            "/i/local/nonexistent.jpg/output:f=jpg",
			expectedStatus: fiber.StatusInternalServerError,
			expectedType:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			resp, err := app.Test(req, 10*1000) // 10 second timeout
			if err != nil {
				t.Fatalf("Failed to perform request: %v", err)
			}

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			if tt.expectedStatus == fiber.StatusOK {
				// Check content type
				contentType := resp.Header.Get("Content-Type")
				if contentType != tt.expectedType {
					t.Errorf("Expected content type %s, got %s", tt.expectedType, contentType)
				}

				// Check that we got image data
				if resp.ContentLength == 0 {
					t.Error("Expected image data but got empty response")
				}

				// Check cache control header
				cacheControl := resp.Header.Get("Cache-Control")
				if cacheControl == "" {
					t.Error("Expected Cache-Control header to be set")
				}
			}
		})
	}
}

func TestIntegration_ConcurrentRequests(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "thumbla_integration_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test image
	jpegData, err := createTestImage(100, 100, "jpeg")
	if err != nil {
		t.Fatalf("Failed to create test JPEG: %v", err)
	}

	err = os.WriteFile(filepath.Join(tempDir, "concurrent.jpg"), jpegData, 0644)
	if err != nil {
		t.Fatalf("Failed to write test JPEG: %v", err)
	}

	app := createTestApp(tempDir)

	// Test concurrent requests
	const numRequests = 10
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			req := httptest.NewRequest("GET", "/i/local/concurrent.jpg/resize:w=50/output:f=jpg", nil)
			resp, err := app.Test(req, 5*1000)
			if err != nil {
				results <- err
				return
			}

			if resp.StatusCode != fiber.StatusOK {
				results <- err
				return
			}

			results <- nil
		}(i)
	}

	// Wait for all requests to complete
	timeout := time.After(10 * time.Second)
	successCount := 0

	for i := 0; i < numRequests; i++ {
		select {
		case err := <-results:
			if err == nil {
				successCount++
			} else {
				t.Errorf("Request %d failed: %v", i, err)
			}
		case <-timeout:
			t.Fatalf("Timeout waiting for concurrent requests")
		}
	}

	if successCount != numRequests {
		t.Errorf("Expected %d successful requests, got %d", numRequests, successCount)
	}
}