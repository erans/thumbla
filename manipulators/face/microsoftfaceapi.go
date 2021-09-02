package face

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"io/ioutil"
	"net/http"

	"github.com/erans/thumbla/config"
	"github.com/labstack/echo/v4"
)

const (
	// MicrosoftFaceAPI uses the Microsoft Face detection API
	MicrosoftFaceAPI = "microsoftFaceAPI"
)

type microsoftFace struct {
	FaceID        string `json:"faceId"`
	FaceRectangle struct {
		Top    int `json:"top"`
		Left   int `json:"left"`
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"faceRectangle"`
}

// MicrosoftFaceAPIDetector provides facial recognition using Microsoft Face API
type MicrosoftFaceAPIDetector struct {
}

// NewMicrosoftFaceAPIDetector returns a new face detector using Microsoft's Face API
func NewMicrosoftFaceAPIDetector() *MicrosoftFaceAPIDetector {
	return &MicrosoftFaceAPIDetector{}
}

// Detect uses Microsoft Face API to detect faces in images
func (d *MicrosoftFaceAPIDetector) Detect(c echo.Context, cfg *config.Config, params map[string]string, img image.Image) ([]image.Rectangle, error) {
	c.Logger().Debugf("Detecting using Microsoft Face API")
	buf := new(bytes.Buffer)
	var err = jpeg.Encode(buf, img, nil)
	if err != nil {
		return nil, err
	}

	var req *http.Request
	if req, err = http.NewRequest("POST", cfg.FaceAPI.MicrosoftFaceAPI.URL, buf); err != nil {
		return nil, err
	}

	c.Logger().Debugf("MS Face API URL: %s", cfg.FaceAPI.MicrosoftFaceAPI.URL)
	req.Header.Set("Ocp-Apim-Subscription-Key", cfg.FaceAPI.MicrosoftFaceAPI.Key)
	req.Header.Set("Content-Type", "application/octet-stream")

	var resp *http.Response
	client := &http.Client{}
	if resp, err = client.Do(req); err != nil {
		return nil, err
	}

	body, _ := ioutil.ReadAll(resp.Body)
	c.Logger().Debugf("status code: %v", resp.StatusCode)
	c.Logger().Debugf("body: %v", string(body))

	tempFaces := []microsoftFace{}
	err = json.Unmarshal(body, &tempFaces)
	if err != nil {
		return nil, err
	}

	c.Logger().Debugf("Faces: %v", tempFaces)

	faces := make([]image.Rectangle, len(tempFaces))
	for i, v := range tempFaces {
		faces[i] = image.Rect(v.FaceRectangle.Left, v.FaceRectangle.Top, v.FaceRectangle.Left+v.FaceRectangle.Width, v.FaceRectangle.Top+v.FaceRectangle.Height)
	}

	return faces, nil
}
