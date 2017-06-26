package face

import (
	"bytes"
	"image"
	"image/jpeg"

	"github.com/erans/thumbla/config"
	"github.com/labstack/echo"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rekognition"
)

const (
	// AWSRekognitionAPI uses AWS Rekognition API that has, among other things, a facial detection API.
	AWSRekognitionAPI = "awsRekognition"
)

// AWSRekognitionDetector provides facial detection using AWS's Rekognition API
type AWSRekognitionDetector struct{}

// NewAWSRekognitionDetector returns a new AWS Rekognition based facial detector
func NewAWSRekognitionDetector() *AWSRekognitionDetector {
	return &AWSRekognitionDetector{}
}

// Detect uses AWS Rekognition API to detect faces in images
func (d *AWSRekognitionDetector) Detect(c echo.Context, cfg *config.Config, params map[string]string, img image.Image) ([]image.Rectangle, error) {
	c.Logger().Debugf("Detecting using AWS Rekognition")
	var config *aws.Config

	if cfg.FaceAPI.AWSRekognition.Region != "" {
		config = &aws.Config{
			Region: &cfg.FaceAPI.AWSRekognition.Region,
		}
	}

	sess := session.Must(session.NewSession(config))
	svc := rekognition.New(sess)

	buf := new(bytes.Buffer)
	var err = jpeg.Encode(buf, img, nil)
	if err != nil {
		return nil, err
	}

	var detectAttributeDefault = "DEFAULT"

	input := &rekognition.DetectFacesInput{
		Attributes: []*string{
			&detectAttributeDefault,
		},
		Image: &rekognition.Image{
			Bytes: buf.Bytes(),
		},
	}

	var output *rekognition.DetectFacesOutput
	if output, err = svc.DetectFaces(input); err != nil {
		return nil, err
	}

	c.Logger().Debugf("AWS Rekognition response: %v", output)

	if output != nil && len(output.FaceDetails) > 0 {
		var result = make([]image.Rectangle, len(output.FaceDetails))

		imgWidth := float64(img.Bounds().Dx())
		imgHeight := float64(img.Bounds().Dy())

		for i, f := range output.FaceDetails {
			top := int(*f.BoundingBox.Top * imgHeight)
			left := int(*f.BoundingBox.Left * imgWidth)
			width := int(*f.BoundingBox.Width * imgWidth)
			height := int(*f.BoundingBox.Height * imgHeight)
			result[i] = image.Rect(left, top, left+width, top+height)
		}

		if len(result) > 0 {
			return result, nil
		}
	}

	return nil, nil
}
