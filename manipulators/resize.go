package manipulators

import (
	"fmt"
	"image"
	"math"
	"strconv"

	"github.com/anthonynsimon/bild/transform"
	"github.com/erans/thumbla/config"
	"github.com/erans/thumbla/utils"
	"github.com/labstack/echo/v4"
)

// ResizeManipulator resizes the image based on given parameters. If only 1 parameter is given, proportions are saved
type ResizeManipulator struct {
}

var resamplingFilters = map[string]transform.ResampleFilter{
	"nearest":           transform.NearestNeighbor,
	"box":               transform.Box,
	"linear":            transform.Linear,
	"gaussian":          transform.Gaussian,
	"mitchellnetravali": transform.MitchellNetravali,
	"catmullrom":        transform.CatmullRom,
	"lanczos":           transform.Lanczos,
}

// Execute runs the resize manipulator and resizes the image. If only width or height are specified, image proportions will be saved
// w - Width
// h - Height
// r - resampling filter (one of resamplingFilters values)
func (manipulator *ResizeManipulator) Execute(c echo.Context, params map[string]string, img image.Image) (image.Image, error) {
	var width = -1.0
	var height = -1.0
	var resamplingFilter = transform.Linear
	var temp string
	var err error
	var ok bool

	// See if a resampling filter was set (default is Linear)
	if temp, ok = params["r"]; ok {
		if f, exists := resamplingFilters[temp]; exists {
			resamplingFilter = f
		}
	}

	if temp, ok = params["w"]; ok {
		if width, err = strconv.ParseFloat(temp, 64); err != nil {
			return nil, fmt.Errorf("Failed to convert width parameter")
		}
	}

	if temp, ok = params["h"]; ok {
		if height, err = strconv.ParseFloat(temp, 64); err != nil {
			return nil, fmt.Errorf("Failed to convert height parameter")
		}
	}

	if width < 0 && height < 0 {
		return nil, fmt.Errorf("Both width and height are less than 0")
	}

	imgWidth := img.Bounds().Size().X
	imgHeight := img.Bounds().Size().Y
	var ratio = math.Max(float64(imgWidth), float64(imgHeight)) / math.Min(float64(imgWidth), float64(imgHeight))

	c.Logger().Debugf("resize: ratio=%f", ratio)

	if imgWidth > imgHeight {
		if width == -1 {
			width = utils.Round(float64(height)*ratio, .5, 0)
		}

		if height == -1 {
			height = utils.Round(float64(width)/ratio, .5, 0)
		}
	} else {
		if width == -1 {
			width = utils.Round(float64(height)/ratio, .5, 0)
		}

		if height == -1 {
			height = utils.Round(float64(width)*ratio, .5, 0)
		}
	}

	c.Logger().Debugf("Original w: %d  h: %d   New w: %d h: %d  %fx%f", imgWidth, imgHeight, int(width), int(height), width, height)

	img = transform.Resize(img, int(width), int(height), resamplingFilter)

	return img, nil
}

// NewResizeManipulator returns a new Resize Manipulator
func NewResizeManipulator(cfg *config.Config) *ResizeManipulator {
	return &ResizeManipulator{}
}
