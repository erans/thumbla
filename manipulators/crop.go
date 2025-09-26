package manipulators

import (
	"fmt"
	"image"
	"strconv"
	"strings"

	"github.com/anthonynsimon/bild/transform"
	"github.com/erans/thumbla/config"
	"github.com/gofiber/fiber/v2"
)

// CropManipulator crops the image
type CropManipulator struct {
}

// Execute runs the crop manipulator and crops the image
func (manipulator *CropManipulator) Execute(c *fiber.Ctx, params map[string]string, img image.Image) (image.Image, error) {
	// Handle x, y, w, h parameters (standard crop)
	if x, hasX := params["x"]; hasX {
		y, hasY := params["y"]
		w, hasW := params["w"]
		h, hasH := params["h"]

		if !hasY || !hasW || !hasH {
			return img, fmt.Errorf("crop requires x, y, w, h parameters")
		}

		xVal, xerr := strconv.Atoi(x)
		yVal, yerr := strconv.Atoi(y)
		wVal, werr := strconv.Atoi(w)
		hVal, herr := strconv.Atoi(h)

		if xerr != nil || yerr != nil || werr != nil || herr != nil {
			return img, fmt.Errorf("invalid crop parameters: must be integers")
		}

		// Validate bounds
		bounds := img.Bounds()
		if xVal < 0 || yVal < 0 || wVal <= 0 || hVal <= 0 {
			return img, fmt.Errorf("invalid crop parameters: negative or zero values")
		}
		if xVal+wVal > bounds.Max.X || yVal+hVal > bounds.Max.Y {
			return img, fmt.Errorf("crop area exceeds image bounds")
		}

		rectangle := image.Rect(xVal, yVal, xVal+wVal, yVal+hVal)
		return transform.Crop(img, rectangle), nil
	}

	// Handle r parameter (legacy rectangle format)
	if r, ok := params["r"]; ok {
		parts := strings.Split(r, "|")

		// Check if we have 2 parts that both end with %
		if len(parts) == 2 && strings.HasSuffix(parts[0], "%") && strings.HasSuffix(parts[1], "%") {
			// Parse percentage values
			widthStr := strings.TrimSuffix(parts[0], "%")
			heightStr := strings.TrimSuffix(parts[1], "%")

			width, werr := strconv.ParseFloat(widthStr, 64)
			height, herr := strconv.ParseFloat(heightStr, 64)

			if werr != nil || herr != nil {
				return nil, fmt.Errorf("invalid percentage values for crop")
			}

			bounds := img.Bounds()
			x1 := int(float64(bounds.Max.X) * (width / 100.0))
			y1 := int(float64(bounds.Max.Y) * (height / 100.0))

			rectangle := image.Rect(0, 0, x1, y1)
			return transform.Crop(img, rectangle), nil
		}

		if len(parts) == 2 {
			return nil, fmt.Errorf("crop rectangle (r) passed 2 values must be a percentages of the width and the percentage of the height")
		}

		if len(parts) < 3 {
			return nil, fmt.Errorf("crop rectangle (r) must have 4 values separating by a '|' sign")
		}

		x0, x0err := strconv.Atoi(parts[0])
		y0, y0err := strconv.Atoi(parts[1])
		x1, x1err := strconv.Atoi(parts[2])
		y1, y1err := strconv.Atoi(parts[3])

		if x0err != nil || y0err != nil || x1err != nil || y1err != nil {
			return nil, fmt.Errorf("one of the crop rectangle (r) values is invalid")
		}

		fmt.Printf("%d %d %d %d", x0, y0, x1, y1)

		bounds := img.Bounds()
		if x1 < 0 {
			x1 = bounds.Max.X + x1
		}
		if y1 < 0 {
			y1 = bounds.Max.Y + y1
		}

		rectangle := image.Rect(x0, y0, x1, y1)

		return transform.Crop(img, rectangle), nil
	}

	return img, nil
}

// NewCropManipulator returns a new crop Manipulator
func NewCropManipulator(cfg *config.Config) *CropManipulator {
	return &CropManipulator{}
}
