package manipulators

import (
	"fmt"
	"image"
	"strconv"

	"github.com/anthonynsimon/bild/adjust"
	"github.com/erans/thumbla/config"
	"github.com/labstack/echo/v4"
)

// BrightnessManipulator adjusts the brightness of the image
type BrightnessManipulator struct {
}

// Execute runs the brightness manipulator and adjusts the image brightness
func (manipulator *BrightnessManipulator) Execute(c echo.Context, params map[string]string, img image.Image) (image.Image, error) {
	if brightnessStr, ok := params["v"]; ok {
		brightness, err := strconv.ParseFloat(brightnessStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid brightness value: %v", err)
		}

		// Clamp brightness value between -100 and 100
		if brightness < -100 {
			brightness = -100
		} else if brightness > 100 {
			brightness = 100
		}

		// Convert from percentage (-100 to 100) to factor (-1 to 1)
		brightnessFactor := brightness / 100.0

		return adjust.Brightness(img, brightnessFactor), nil
	}

	return img, nil
}

// NewBrightnessManipulator returns a new brightness Manipulator
func NewBrightnessManipulator(cfg *config.Config) *BrightnessManipulator {
	return &BrightnessManipulator{}
}
