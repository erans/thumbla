package face

import (
	"image"

	"github.com/erans/thumbla/config"
	"github.com/gofiber/fiber/v2"
)

// Detector provides a single interface to access various facial recognition APIs
type Detector interface {
	Detect(c *fiber.Ctx, cfg *config.Config, params map[string]string, img image.Image) ([]image.Rectangle, error)
}

var detectorRegistry = map[string]Detector{
	MicrosoftFaceAPI:     NewMicrosoftFaceAPIDetector(),
	GoogleCloudVisionAPI: NewGoogleCloudVisionAPIDetector(),
	AWSRekognitionAPI:    NewAWSRekognitionDetector(),
}

// GetDetectorByName returns a detector by its name
func GetDetectorByName(name string) Detector {
	if d, ok := detectorRegistry[name]; ok {
		return d
	}

	return nil
}
