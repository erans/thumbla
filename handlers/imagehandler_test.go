package handlers

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/erans/thumbla/config"
	"github.com/erans/thumbla/fetchers"
	"github.com/erans/thumbla/manipulators"
	"github.com/gofiber/fiber/v2"
)

func createTestImage(width, height int, format string) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with red color
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
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

func setupTestEnvironment(t *testing.T) (string, func()) {
	// Create temporary directory for test images
	tempDir, err := os.MkdirTemp("", "thumbla_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create test images
	jpegData, err := createTestImage(100, 100, "jpeg")
	if err != nil {
		t.Fatalf("Failed to create test JPEG: %v", err)
	}

	pngData, err := createTestImage(100, 100, "png")
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

	// Setup configuration
	cfg := &config.Config{
		Fetchers: []map[string]interface{}{
			{
				"name": "local",
				"type": "local",
				"path": tempDir,
			},
		},
		Paths: []config.PathConfig{
			{
				Path:        "/test",
				FetcherName: "local",
			},
		},
	}

	// Initialize global config
	config.SetConfig(cfg)

	// Initialize fetchers and manipulators
	fetchers.InitFetchers(cfg)
	manipulators.InitManipulators(cfg)

	return tempDir, func() {
		os.RemoveAll(tempDir)
	}
}

func TestHandleImage_BasicFetch(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	app := fiber.New()
	app.Get("/test/:url/*", HandleImage)

	tests := []struct {
		name           string
		url            string
		expectedStatus int
	}{
		{
			name:           "fetch existing JPEG",
			url:            "/test/test.jpg/output:f=jpg",
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "fetch existing PNG",
			url:            "/test/test.png/output:f=png",
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "fetch non-existent file",
			url:            "/test/nonexistent.jpg/output:f=jpg",
			expectedStatus: fiber.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Failed to perform request: %v", err)
			}

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			if tt.expectedStatus == fiber.StatusOK {
				// Check content type
				contentType := resp.Header.Get("Content-Type")
				if contentType == "" {
					t.Error("Expected Content-Type header to be set")
				}
			}
		})
	}
}

func TestHandleImage_Manipulations(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	app := fiber.New()
	app.Get("/test/:url/*", HandleImage)

	tests := []struct {
		name           string
		url            string
		expectedStatus int
	}{
		{
			name:           "resize image",
			url:            "/test/test.jpg/resize:w=50/output:f=jpg",
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "crop image",
			url:            "/test/test.jpg/crop:x=10,y=10,w=50,h=50/output:f=jpg",
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "rotate image",
			url:            "/test/test.jpg/rotate:a=90/output:f=jpg",
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "multiple manipulations",
			url:            "/test/test.jpg/resize:w=80/rotate:a=45/output:f=jpg",
			expectedStatus: fiber.StatusOK,
		},
		{
			name:           "format conversion",
			url:            "/test/test.jpg/output:f=png",
			expectedStatus: fiber.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Failed to perform request: %v", err)
			}

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			if tt.expectedStatus == fiber.StatusOK {
				// Verify we get image data back
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Errorf("Failed to read response body: %v", err)
				}
				if len(body) == 0 {
					t.Error("Expected image data in response body")
				}
			}
		})
	}
}

func TestHandleImage_InvalidRequests(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	app := fiber.New()
	app.Get("/test/:url/*", HandleImage)
	app.Get("/undefined/:url/*", HandleImage) // Route with no fetcher defined

	tests := []struct {
		name           string
		url            string
		expectedStatus int
	}{
		{
			name:           "invalid URL encoding",
			url:            "/test/test%20file/output:f=jpg",
			expectedStatus: fiber.StatusInternalServerError, // Will fail when trying to fetch non-existent file
		},
		{
			name:           "no fetcher for path",
			url:            "/undefined/test.jpg/output:f=jpg", // This will use /undefined path which has no fetcher defined
			expectedStatus: fiber.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Failed to perform request: %v", err)
			}

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
		})
	}
}