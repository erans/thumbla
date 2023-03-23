package manipulators

import (
	"image"

	"github.com/erans/thumbla/config"
	"github.com/labstack/echo/v4"
)

// Manipulator interface
type Manipulator interface {
	Execute(c echo.Context, params map[string]string, img image.Image) (image.Image, error)
}

var manipulatorsRegistry map[string]Manipulator

// GetManipulatorByName returns a manipulator from the registered manipulators list
func GetManipulatorByName(name string) Manipulator {
	if manipulatorsRegistry != nil {
		if manipulator, ok := manipulatorsRegistry[name]; ok {
			return manipulator
		}
	}

	return nil
}

// InitManipulators initializes registered manipulators
func InitManipulators(cfg *config.Config) {
	manipulatorsRegistry = map[string]Manipulator{
		// transformation manipulators
		"output":   NewOutputManipulator(cfg),
		"rotate":   NewRotateManipulator(cfg),
		"flipv":    NewFlipVerticalManipulator(cfg),
		"fliph":    NewFlipHorizontalManipulator(cfg),
		"resize":   NewResizeManipulator(cfg),
		"fit":      NewFitManipulator(cfg),
		"crop":     NewCropManipulator(cfg),
		"shearv":   NewShearVerticalManipulator(cfg),
		"shearh":   NewShearHorizontalManipulator(cfg),
		"facecrop": NewFaceCropManipulator(cfg),
		"paste":    NewPasteManipulator(cfg),
	}
}
