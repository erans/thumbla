package face

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/jpeg"
	"net/http"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi/transport"
	vision "google.golang.org/api/vision/v1"

	"github.com/erans/thumbla/config"
	"github.com/labstack/echo/v4"
)

const (
	// GoogleCloudVisionAPI uses Google's Cloud Vision API that has, among other things, a facial detection API.
	GoogleCloudVisionAPI = "googleCloudVisionAPI"
)

// GoogleCloudVisionAPIDetector provides facial detection using Google's Vision API
type GoogleCloudVisionAPIDetector struct {
}

// NewGoogleCloudVisionAPIDetector returns a new Google Cloud Vision API detector
func NewGoogleCloudVisionAPIDetector() *GoogleCloudVisionAPIDetector {
	return &GoogleCloudVisionAPIDetector{}
}

// Detect finds faces using Google's Vision API
func (d *GoogleCloudVisionAPIDetector) Detect(c echo.Context, cfg *config.Config, params map[string]string, img image.Image) ([]image.Rectangle, error) {
	c.Logger().Debugf("Detecting using Google Vision API")
	var err error
	var client *http.Client
	ctx := context.Background()

	if cfg.FaceAPI.GoogleCloudVisionAPI.Key != "" {
		client = &http.Client{
			Transport: &transport.APIKey{Key: cfg.FaceAPI.GoogleCloudVisionAPI.Key},
		}
	} else {
		if client, err = google.DefaultClient(ctx, vision.CloudPlatformScope); err != nil {
			return nil, err
		}
	}

	var service *vision.Service
	if service, err = vision.New(client); err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, img, nil)
	if err != nil {
		return nil, err
	}

	req := &vision.AnnotateImageRequest{
		Image: &vision.Image{
			Content: base64.StdEncoding.EncodeToString(buf.Bytes()),
		},
		Features: []*vision.Feature{
			{
				Type:       "FACE_DETECTION",
				MaxResults: 10,
			},
		},
	}

	batch := &vision.BatchAnnotateImagesRequest{
		Requests: []*vision.AnnotateImageRequest{req},
	}

	var res *vision.BatchAnnotateImagesResponse
	res, err = service.Images.Annotate(batch).Do()
	if err != nil {
		c.Logger().Errorf("Google Vision API request failed: %v", err)
		return nil, err
	}

	if faceAnnotations := res.Responses[0].FaceAnnotations; len(faceAnnotations) > 0 {
		var result = make([]image.Rectangle, len(faceAnnotations))
		for i, f := range faceAnnotations {
			c.Logger().Debugf("Bounding Poly: %v", f.BoundingPoly.Vertices)
			x0 := int(f.BoundingPoly.Vertices[0].X)
			y0 := int(f.BoundingPoly.Vertices[0].Y)

			x1 := int(f.BoundingPoly.Vertices[2].X)
			y1 := int(f.BoundingPoly.Vertices[2].Y)

			result[i] = image.Rect(x0, y0, x1, y1)
		}

		if result != nil && len(result) > 0 {
			return result, nil
		}
	}

	return nil, nil
}
