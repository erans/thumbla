package manipulators

import (
	"fmt"
	"image"
	"strings"

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
						c.Locals("X-Quality", val)
					}
				}
			}

			if contentType == "image/webp" {
				if val, ok := params["lossless"]; ok {
					if c != nil {
						c.Locals("X-Lossless", val)
					}
				}

				if val, ok := params["exact"]; ok {
					if c != nil {
						c.Locals("X-Exact", val)
					}
				}
			}

			if val, ok := params["e"]; ok {
				if c != nil {
					logger := middleware.GetLoggerFromContext(c)
					logger.Debug().Str("encoder", val).Msg("Setting encoder")
					c.Locals("X-Encoder", val)
				}
			}
		} else {
			return nil, fmt.Errorf("invalid or unsupported content type format '%s'", val)
		}

	}

	return img, nil
}

func (manipulator *OutputManipulator) Validate(params map[string]string) error {
	if val, ok := params["f"]; ok {
		if _, ok := formatContentTypeMapping[strings.ToLower(val)]; !ok {
			return fmt.Errorf("unsupported format: %s", val)
		}
	}
	if v, ok := params["q"]; ok {
		if err := ValidateNumericInt("q", v, 1, 100); err != nil {
			return err
		}
	}
	for _, b := range []string{"lossless", "exact", "progressive"} {
		if v, ok := params[b]; ok {
			if v != "true" && v != "false" && v != "1" && v != "0" {
				return fmt.Errorf("parameter %s requires boolean value, got: %s", b, v)
			}
		}
	}
	return nil
}

// NewOutputManipulator returns a new Output Manipulator
func NewOutputManipulator(cfg *config.Config) *OutputManipulator {
	return &OutputManipulator{}
}
