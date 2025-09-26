package manipulators

import (
	"image"

	"github.com/anthonynsimon/bild/transform"
	"github.com/erans/thumbla/config"
	"github.com/gofiber/fiber/v2"
)

// FlipHorizontalManipulator flips the image horizontally
type FlipHorizontalManipulator struct {
}

// Execute runs the flip horizontal manipulator and flips the image horizontally
func (manipulator *FlipHorizontalManipulator) Execute(c *fiber.Ctx, params map[string]string, img image.Image) (image.Image, error) {
	return transform.FlipH(img), nil
}

// NewFlipHorizontalManipulator returns a new flip horizontal Manipulator
func NewFlipHorizontalManipulator(cfg *config.Config) *FlipHorizontalManipulator {
	return &FlipHorizontalManipulator{}
}
