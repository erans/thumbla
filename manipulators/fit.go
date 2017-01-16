package manipulators

import (
	"fmt"
	"image"
	"strconv"

	"github.com/erans/thumbla/config"
	"github.com/labstack/echo"
)

// FitManipulator fits the image to the specified size
type FitManipulator struct {
}

// Execute runs the fit manipulator and fits the image to the specified size
func (manipulator *FitManipulator) Execute(c echo.Context, params map[string]string, img image.Image) (image.Image, error) {
	var maxW = -1
	var maxH = -1
	var resizeFilterName = "linear"
	var err error

	if v, ok := params["w"]; ok {
		c.Logger().Debugf("Fit: W=%s", v)
		if maxW, err = strconv.Atoi(v); err != nil {
			return nil, fmt.Errorf("Invalid width (w) value")
		}
	}

	if v, ok := params["h"]; ok {
		if maxH, err = strconv.Atoi(v); err != nil {
			return nil, fmt.Errorf("Invalid height (h) value")
		}
	}

	if v, ok := params["r"]; ok {
		resizeFilterName = v
	}

	srcBounds := img.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	if srcW <= 0 || srcH <= 0 {
		return nil, fmt.Errorf("Invalid width or height of source image")
	}

	srcRatio := float64(srcW) / float64(srcH)
	maxRatio := float64(maxW) / float64(maxH)

	var newW, newH int
	if srcRatio > maxRatio {
		newW = maxW
		newH = int(float64(newW) / srcRatio)
	} else {
		newH = maxH
		newW = int(float64(newH) * srcRatio)
	}

	resizeManipulator := GetManipulatorByName("resize")
	resizeParams := map[string]string{
		"w": strconv.Itoa(newW),
		"h": strconv.Itoa(newH),
		"r": resizeFilterName,
	}

	return resizeManipulator.Execute(c, resizeParams, img)
}

// NewFitManipulator returns a new fit Manipulator
func NewFitManipulator(cfg *config.Config) *FitManipulator {
	return &FitManipulator{}
}
