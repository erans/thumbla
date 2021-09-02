package manipulators

import (
	"image"
	"strconv"

	"github.com/anthonynsimon/bild/transform"
	"github.com/erans/thumbla/config"
	"github.com/labstack/echo/v4"
)

// ShearVerticalManipulator shears the image vertically
type ShearVerticalManipulator struct {
}

// Execute runs the shear horizontal manipulator and shears the image horizontally
func (manipulator *ShearVerticalManipulator) Execute(c echo.Context, params map[string]string, img image.Image) (image.Image, error) {
	if a, ok := params["a"]; ok {
		if angle, err := strconv.ParseFloat(a, 64); err == nil {
			return transform.ShearV(img, angle), nil
		}
	}

	return img, nil
}

// NewShearVerticalManipulator returns a new shear vertical Manipulator
func NewShearVerticalManipulator(cfg *config.Config) *ShearVerticalManipulator {
	return &ShearVerticalManipulator{}
}
