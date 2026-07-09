package manipulators

import (
	"fmt"
	"image"
	"strconv"

	"github.com/erans/thumbla/config"
	"github.com/gofiber/fiber/v2"
)

// Manipulator interface
type Manipulator interface {
	Execute(c *fiber.Ctx, params map[string]string, img image.Image) (image.Image, error)
	Validate(params map[string]string) error
}

// ValidateParamShape enforces generic parameter-name/value limits before
// per-manipulator semantic validation runs.
func ValidateParamShape(name, value string) error {
	if len(name) > 50 {
		return fmt.Errorf("parameter name too long: %d characters", len(name))
	}
	if len(value) > 100 {
		return fmt.Errorf("parameter value too long: %d characters", len(value))
	}
	return nil
}

// ValidateNumericInt parses a param as an int and checks bounds.
func ValidateNumericInt(name, value string, min, max int) error {
	v, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("parameter %s requires integer value, got: %s", name, value)
	}
	if v < min || v > max {
		return fmt.Errorf("parameter %s value %d outside [%d, %d]", name, v, min, max)
	}
	return nil
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
		"output":     NewOutputManipulator(cfg),
		"rotate":     NewRotateManipulator(cfg),
		"flipv":      NewFlipVerticalManipulator(cfg),
		"fliph":      NewFlipHorizontalManipulator(cfg),
		"resize":     NewResizeManipulator(cfg),
		"fit":        NewFitManipulator(cfg),
		"crop":       NewCropManipulator(cfg),
		"shearv":     NewShearVerticalManipulator(cfg),
		"shearh":     NewShearHorizontalManipulator(cfg),
		"facecrop":   NewFaceCropManipulator(cfg),
		"paste":      NewPasteManipulator(cfg),
		"contrast":   NewContrastManipulator(cfg),
		"brightness": NewBrightnessManipulator(cfg),
	}
}
