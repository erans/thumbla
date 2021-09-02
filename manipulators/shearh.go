package manipulators

import (
	"image"
	"strconv"

	"github.com/anthonynsimon/bild/transform"
	"github.com/erans/thumbla/config"
	"github.com/labstack/echo/v4"
)

// ShearHorizontalManipulator shears the image horizontally
type ShearHorizontalManipulator struct {
}

// Execute runs the shear horizontal manipulator and shears the image horizontally
func (manipulator *ShearHorizontalManipulator) Execute(c echo.Context, params map[string]string, img image.Image) (image.Image, error) {
	if a, ok := params["a"]; ok {
		if angle, err := strconv.ParseFloat(a, 64); err == nil {
			return transform.ShearH(img, angle), nil
		}
	}

	return img, nil
}

// NewShearHorizontalManipulator returns a new shear horizontal Manipulator
func NewShearHorizontalManipulator(cfg *config.Config) *ShearHorizontalManipulator {
	return &ShearHorizontalManipulator{}
}
