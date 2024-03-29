package manipulators

import (
	"fmt"
	"image"

	"github.com/erans/thumbla/config"
	"github.com/labstack/echo/v4"
)

var formatContentTypeMapping = map[string]string{
	"jpg":  "image/jpg",
	"jpeg": "image/jpeg",
	"png":  "image/png",
	"webp": "image/webp",
}

// OutputManipulator sets the content-type that will be used as the output for the image processing format
type OutputManipulator struct {
}

// Execute runs the output format manipulator, setting the content-type that will be used to save the resulting image
func (manipulator *OutputManipulator) Execute(c echo.Context, params map[string]string, img image.Image) (image.Image, error) {
	if val, ok := params["f"]; ok {
		if contentType, ok := formatContentTypeMapping[val]; ok {
			c.Response().Header().Set("Content-Type", contentType)

			if contentType == "image/jpeg" || contentType == "image/jpg" || contentType == "image/webp" {
				if val, ok := params["q"]; ok {
					c.Response().Header().Set("X-Quality", val)
				}
			}

			if contentType == "image/web" {
				if val, ok := params["lossless"]; ok {
					c.Response().Header().Set("X-Lossless", val)
				}

				if val, ok := params["exact"]; ok {
					c.Response().Header().Set("X-Exact", val)
				}
			}

			if val, ok := params["e"]; ok {
				c.Logger().Debugf("Encoder: %s", val)
				if val == "guetzli" {
					c.Response().Header().Set("X-Encoder", val)
				}
			}
		} else {
			return nil, fmt.Errorf("invalid or unsupported content type format '%s'", contentType)
		}

	}

	return img, nil
}

// NewOutputManipulator returns a new Output Manipulator
func NewOutputManipulator(cfg *config.Config) *OutputManipulator {
	return &OutputManipulator{}
}
