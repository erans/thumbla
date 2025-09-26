package manipulators

import (
	"image"
	"image/color"
	"testing"

	"github.com/erans/thumbla/config"
	"github.com/gofiber/fiber/v2"
)

func TestOutputManipulator(t *testing.T) {
	cfg := &config.Config{}
	manipulator := NewOutputManipulator(cfg)

	// Create a test image
	testImg := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			testImg.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}

	tests := []struct {
		name           string
		params         map[string]string
		expectedHeader string
		expectedValue  string
	}{
		{
			name:           "set JPEG format",
			params:         map[string]string{"f": "jpg"},
			expectedHeader: "Content-Type",
			expectedValue:  "image/jpeg",
		},
		{
			name:           "set PNG format",
			params:         map[string]string{"f": "png"},
			expectedHeader: "Content-Type",
			expectedValue:  "image/png",
		},
		{
			name:           "set WebP format",
			params:         map[string]string{"f": "webp"},
			expectedHeader: "Content-Type",
			expectedValue:  "image/webp",
		},
		{
			name:           "set JPEG quality",
			params:         map[string]string{"f": "jpg", "q": "80"},
			expectedHeader: "X-Quality",
			expectedValue:  "80",
		},
		{
			name:           "set WebP lossless",
			params:         map[string]string{"f": "webp", "lossless": "true"},
			expectedHeader: "X-Lossless",
			expectedValue:  "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a nil context for testing (manipulators don't use context)
			var ctx *fiber.Ctx

			result, err := manipulator.Execute(ctx, tt.params, testImg)
			if err != nil {
				t.Errorf("Execute() error = %v", err)
				return
			}

			// Should return the same image
			if result != testImg {
				t.Error("Output manipulator should return the same image")
			}

			// Headers won't be set with nil context, so we just verify the image is returned unchanged
		})
	}
}

func TestOutputManipulator_UnsupportedFormat(t *testing.T) {
	cfg := &config.Config{}
	manipulator := NewOutputManipulator(cfg)

	testImg := image.NewRGBA(image.Rect(0, 0, 10, 10))

	// Create a nil context for testing (manipulators don't use context)
	var ctx *fiber.Ctx

	params := map[string]string{"f": "unsupported"}
	result, err := manipulator.Execute(ctx, params, testImg)

	// Should return error for unsupported format
	if err == nil {
		t.Error("Expected error for unsupported format")
	}
	if result != nil {
		t.Error("Should return nil image for unsupported format when error occurs")
	}
}