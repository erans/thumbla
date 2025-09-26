package manipulators

import (
	"image"
	"image/color"
	"testing"

	"github.com/erans/thumbla/config"
	"github.com/gofiber/fiber/v2"
)

func TestRotateManipulator(t *testing.T) {
	cfg := &config.Config{}
	manipulator := NewRotateManipulator(cfg)

	// Create a test image (10x10 red square)
	testImg := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			testImg.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}

	tests := []struct {
		name   string
		params map[string]string
	}{
		{
			name:   "rotate 90 degrees",
			params: map[string]string{"a": "90"},
		},
		{
			name:   "rotate 180 degrees",
			params: map[string]string{"a": "180"},
		},
		{
			name:   "rotate 270 degrees",
			params: map[string]string{"a": "270"},
		},
		{
			name:   "rotate 45 degrees",
			params: map[string]string{"a": "45"},
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

			// Ensure we get a valid image back
			if result == nil {
				t.Error("Execute() returned nil image")
			}

			// For 90/270 degree rotations, dimensions should be swapped
			if tt.params["a"] == "90" || tt.params["a"] == "270" {
				bounds := result.Bounds()
				if bounds.Dx() != testImg.Bounds().Dy() || bounds.Dy() != testImg.Bounds().Dx() {
					t.Errorf("90/270 degree rotation should swap dimensions")
				}
			}
		})
	}
}

func TestRotateManipulator_InvalidParams(t *testing.T) {
	cfg := &config.Config{}
	manipulator := NewRotateManipulator(cfg)

	testImg := image.NewRGBA(image.Rect(0, 0, 10, 10))

	tests := []struct {
		name   string
		params map[string]string
	}{
		{
			name:   "missing angle",
			params: map[string]string{},
		},
		{
			name:   "invalid angle",
			params: map[string]string{"a": "invalid"},
		},
		{
			name:   "very large angle",
			params: map[string]string{"a": "36000"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a nil context for testing (manipulators don't use context)
			var c *fiber.Ctx

			result, err := manipulator.Execute(c, tt.params, testImg)

			// Should return original image or error for invalid params
			if err == nil && result == nil {
				t.Error("Expected original image or error for invalid params")
			}
		})
	}
}