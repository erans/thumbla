package manipulators

import (
	"fmt"
	"image"
	"strconv"

	"github.com/anthonynsimon/bild/adjust"
	"github.com/erans/thumbla/config"
	"github.com/gofiber/fiber/v2"
)

// ContrastManipulator adjusts the contrast of the image
type ContrastManipulator struct {
}

// Execute runs the contrast manipulator and adjusts the image contrast
func (manipulator *ContrastManipulator) Execute(c *fiber.Ctx, params map[string]string, img image.Image) (image.Image, error) {
	if contrastStr, ok := params["v"]; ok {
		contrast, err := strconv.ParseFloat(contrastStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid contrast value: %v", err)
		}

		// Clamp contrast value between -100 and 100
		if contrast < -100 {
			contrast = -100
		} else if contrast > 100 {
			contrast = 100
		}

		// Convert from percentage (-100 to 100) to factor (-1 to 1)
		contrastFactor := contrast / 100.0

		return adjust.Contrast(img, contrastFactor), nil
	}

	return img, nil
}

// NewContrastManipulator returns a new contrast Manipulator
func NewContrastManipulator(cfg *config.Config) *ContrastManipulator {
	return &ContrastManipulator{}
}
