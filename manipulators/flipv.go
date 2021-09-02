package manipulators

import (
	"image"

	"github.com/anthonynsimon/bild/transform"
	"github.com/erans/thumbla/config"
	"github.com/labstack/echo/v4"
)

// FlipVerticalManipulator flips the image vertically
type FlipVerticalManipulator struct {
}

// Execute runs the flip vertical manipulator and flips the image vertically
func (manipulator *FlipVerticalManipulator) Execute(c echo.Context, params map[string]string, img image.Image) (image.Image, error) {
	return transform.FlipV(img), nil
}

// NewFlipVerticalManipulator returns a new flip vertical Manipulator
func NewFlipVerticalManipulator(cfg *config.Config) *FlipVerticalManipulator {
	return &FlipVerticalManipulator{}
}
