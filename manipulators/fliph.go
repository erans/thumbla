package manipulators

import (
	"image"

	"github.com/anthonynsimon/bild/transform"
	"github.com/erans/thumbla/config"
	"github.com/labstack/echo/v4"
)

// FlipHorizontalManipulator flips the image horizontally
type FlipHorizontalManipulator struct {
}

// Execute runs the flip horizontal manipulator and flips the image horizontally
func (manipulator *FlipHorizontalManipulator) Execute(c echo.Context, params map[string]string, img image.Image) (image.Image, error) {
	return transform.FlipH(img), nil
}

// NewFlipHorizontalManipulator returns a new flip horizontal Manipulator
func NewFlipHorizontalManipulator(cfg *config.Config) *FlipHorizontalManipulator {
	return &FlipHorizontalManipulator{}
}
