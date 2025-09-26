package manipulators

import (
	"image"
	"log"
	"net/url"
	"strconv"
	"strings"

	"github.com/anthonynsimon/bild/transform"
	"github.com/erans/thumbla/config"
	"github.com/gofiber/fiber/v2"
)

// RotateManipulator rotates the image
type RotateManipulator struct {
}

// Execute runs the rotate manipulator and allows rotating an image
// Supported Parameters:
// a - Angle = +/- 360.0 (float)
// r - resize bounds = 0/1
// p - pivot point x|y format
// TODO: support "p" parameter to specify pivot point
func (manipulator *RotateManipulator) Execute(c *fiber.Ctx, params map[string]string, img image.Image) (image.Image, error) {
	var options = &transform.RotationOptions{ResizeBounds: false, Pivot: nil}
	var angle = -1.0

	if val, ok := params["a"]; ok {
		if i, err := strconv.ParseFloat(val, 64); err == nil {
			log.Printf("Rotate at %f degress", i)
			angle = i
		}
	}

	if val, ok := params["r"]; ok {
		if val == "1" {
			options.ResizeBounds = true
		}
	}

	if val, ok := params["p"]; ok {
		val, _ = url.QueryUnescape(val)
		points := strings.Split(val, "|")
		if len(points) > 1 {
			x, xerr := strconv.Atoi(points[0])
			y, yerr := strconv.Atoi(points[1])

			if xerr == nil && yerr == nil {
				log.Printf("Pivot: %d %d", x, y)
				options.Pivot = &image.Point{X: x, Y: y}

				log.Printf("%v", options)
			}
		}
	}

	if angle > -1 {
		rotatedImage := transform.Rotate(img, angle, options)
		return rotatedImage, nil
	}

	return img, nil
}

// NewRotateManipulator returns a new Rotate Manipulator
func NewRotateManipulator(cfg *config.Config) *RotateManipulator {
	return &RotateManipulator{}
}
