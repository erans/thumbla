package manipulators

import (
	"image"
	"image/color"
	"testing"

	"github.com/erans/thumbla/config"
	"github.com/gofiber/fiber/v2"
)

func TestCropManipulator(t *testing.T) {
	cfg := &config.Config{}
	manipulator := NewCropManipulator(cfg)

	// Create a test image (20x20 red square)
	testImg := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			testImg.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}

	tests := []struct {
		name     string
		params   map[string]string
		expected image.Rectangle
	}{
		{
			name:     "crop with x, y, width, height",
			params:   map[string]string{"x": "5", "y": "5", "w": "10", "h": "10"},
			expected: image.Rect(5, 5, 15, 15),
		},
		{
			name:     "crop from origin",
			params:   map[string]string{"x": "0", "y": "0", "w": "15", "h": "15"},
			expected: image.Rect(0, 0, 15, 15),
		},
		{
			name:     "crop at edges",
			params:   map[string]string{"x": "10", "y": "10", "w": "10", "h": "10"},
			expected: image.Rect(10, 10, 20, 20),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a nil context for testing (manipulators don't use context)
			var c *fiber.Ctx

			result, err := manipulator.Execute(c, tt.params, testImg)
			if err != nil {
				t.Errorf("Execute() error = %v", err)
				return
			}

			if result.Bounds() != tt.expected {
				t.Errorf("Execute() bounds = %v, expected %v", result.Bounds(), tt.expected)
			}
		})
	}
}

func TestCropManipulator_InvalidParams(t *testing.T) {
	cfg := &config.Config{}
	manipulator := NewCropManipulator(cfg)

	testImg := image.NewRGBA(image.Rect(0, 0, 20, 20))

	tests := []struct {
		name   string
		params map[string]string
	}{
		{
			name:   "missing width",
			params: map[string]string{"x": "5", "y": "5", "h": "10"},
		},
		{
			name:   "missing height",
			params: map[string]string{"x": "5", "y": "5", "w": "10"},
		},
		{
			name:   "invalid coordinates",
			params: map[string]string{"x": "invalid", "y": "5", "w": "10", "h": "10"},
		},
		{
			name:   "crop outside bounds",
			params: map[string]string{"x": "25", "y": "25", "w": "10", "h": "10"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a nil context for testing (manipulators don't use context)
			var c *fiber.Ctx

			result, err := manipulator.Execute(c, tt.params, testImg)

			// Should return original image or error for invalid params
			if err == nil && result.Bounds() != testImg.Bounds() {
				t.Errorf("Expected original image or error for invalid params, got bounds = %v", result.Bounds())
			}
		})
	}
}