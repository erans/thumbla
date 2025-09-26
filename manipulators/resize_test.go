package manipulators

import (
	"image"
	"image/color"
	"testing"

	"github.com/erans/thumbla/config"
	"github.com/gofiber/fiber/v2"
)

func TestResizeManipulator(t *testing.T) {
	// Create a test config
	cfg := &config.Config{}
	manipulator := NewResizeManipulator(cfg)

	// Create a simple test image (10x10 red square)
	testImg := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			testImg.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}

	tests := []struct {
		name     string
		params   map[string]string
		expected image.Rectangle
	}{
		{
			name:     "resize with width only",
			params:   map[string]string{"w": "20"},
			expected: image.Rect(0, 0, 20, 20),
		},
		{
			name:     "resize with height only",
			params:   map[string]string{"h": "15"},
			expected: image.Rect(0, 0, 15, 15),
		},
		{
			name:     "resize with both width and height",
			params:   map[string]string{"w": "30", "h": "25"},
			expected: image.Rect(0, 0, 30, 25),
		},
		{
			name:     "resize proportionally with width",
			params:   map[string]string{"w": "20", "p": "1"},
			expected: image.Rect(0, 0, 20, 20),
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

func TestResizeManipulator_InvalidParams(t *testing.T) {
	cfg := &config.Config{}
	manipulator := NewResizeManipulator(cfg)

	testImg := image.NewRGBA(image.Rect(0, 0, 10, 10))

	tests := []struct {
		name   string
		params map[string]string
	}{
		{
			name:   "invalid width",
			params: map[string]string{"w": "invalid"},
		},
		{
			name:   "invalid height",
			params: map[string]string{"h": "not_a_number"},
		},
		{
			name:   "zero width",
			params: map[string]string{"w": "0"},
		},
		{
			name:   "negative width",
			params: map[string]string{"w": "-10"},
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