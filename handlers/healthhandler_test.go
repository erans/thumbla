package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestHandleHealth(t *testing.T) {
	// Create a new Fiber app
	app := fiber.New()

	// Add the health route
	app.Get("/health", HandleHealth)

	// Create a test request
	req := httptest.NewRequest("GET", "/health", nil)

	// Perform the request
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to perform request: %v", err)
	}

	// Check status code
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status %d, got %d", fiber.StatusOK, resp.StatusCode)
	}

	// Check response body
	body := make([]byte, 11)
	resp.Body.Read(body)
	if string(body) != "All is well" {
		t.Errorf("Expected body 'All is well', got '%s'", string(body))
	}
}