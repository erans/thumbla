package manipulators

import (
	"fmt"
	"image"
	"strconv"

	"github.com/anthonynsimon/bild/transform"
	"github.com/erans/thumbla/config"
	"github.com/gofiber/fiber/v2"
)

// ShearVerticalManipulator shears the image vertically
type ShearVerticalManipulator struct {
}

// Execute runs the shear horizontal manipulator and shears the image horizontally
func (manipulator *ShearVerticalManipulator) Execute(c *fiber.Ctx, params map[string]string, img image.Image) (image.Image, error) {
	if a, ok := params["a"]; ok {
		if angle, err := strconv.ParseFloat(a, 64); err == nil {
			return transform.ShearV(img, angle), nil
		}
	}

	return img, nil
}

func (manipulator *ShearVerticalManipulator) Validate(params map[string]string) error {
	if v, ok := params["a"]; ok {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("parameter a requires numeric value, got: %s", v)
		}
		if f < -360 || f > 360 {
			return fmt.Errorf("parameter a value %g outside [-360, 360]", f)
		}
	}
	return nil
}

// NewShearVerticalManipulator returns a new shear vertical Manipulator
func NewShearVerticalManipulator(cfg *config.Config) *ShearVerticalManipulator {
	return &ShearVerticalManipulator{}
}
