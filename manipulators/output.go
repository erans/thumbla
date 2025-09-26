package manipulators

import (
	"fmt"
	"image"

	"github.com/erans/thumbla/config"
	"github.com/erans/thumbla/middleware"
	"github.com/gofiber/fiber/v2"
)

var formatContentTypeMapping = map[string]string{
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"png":  "image/png",
	"webp": "image/webp",
}

// OutputManipulator sets the content-type that will be used as the output for the image processing format
type OutputManipulator struct {
}

// Execute runs the output format manipulator, setting the content-type that will be used to save the resulting image
func (manipulator *OutputManipulator) Execute(c *fiber.Ctx, params map[string]string, img image.Image) (image.Image, error) {
	if val, ok := params["f"]; ok {
		if contentType, ok := formatContentTypeMapping[val]; ok {
			if c != nil {
				c.Set("Content-Type", contentType)
			}

			if contentType == "image/jpeg" || contentType == "image/jpg" || contentType == "image/webp" {
				if val, ok := params["q"]; ok {
					if c != nil {
						c.Set("X-Quality", val)
					}
				}
			}

			if contentType == "image/web" {
				if val, ok := params["lossless"]; ok {
					if c != nil {
						c.Set("X-Lossless", val)
					}
				}

				if val, ok := params["exact"]; ok {
					if c != nil {
						c.Set("X-Exact", val)
					}
				}
			}

			if val, ok := params["e"]; ok {
				if c != nil {
				logger := middleware.GetLoggerFromContext(c)
				logger.Debug().Str("encoder", val).Msg("Setting encoder")
			}
				if val == "guetzli" {
					if c != nil {
						c.Set("X-Encoder", val)
					}
				}
			}
		} else {
			return nil, fmt.Errorf("invalid or unsupported content type format '%s'", val)
		}

	}

	return img, nil
}

// NewOutputManipulator returns a new Output Manipulator
func NewOutputManipulator(cfg *config.Config) *OutputManipulator {
	return &OutputManipulator{}
}
