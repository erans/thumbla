package handlers

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/image/webp"

	"github.com/erans/thumbla/fetchers"
	"github.com/erans/thumbla/manipulators"
	"github.com/erans/thumbla/utils"
	"github.com/labstack/echo"
)

type manipulatorAction struct {
	Name   string
	Params map[string]string
}

func loadImage(c echo.Context, url string, contentType string, body io.Reader) (image.Image, error) {
	var image image.Image
	var err error

	c.Logger().Debugf("loadImage contentType=%s", contentType)

	if contentType == "" {
		contentType = utils.GetMimeTypeByFileExt(url)
	}

	if contentType == "" {
		return nil, fmt.Errorf("Content Type is missing and could not be inferred")
	}

	if contentType == "image/jpg" || contentType == "image/jpeg" {
		image, err = jpeg.Decode(body)
	} else if contentType == "image/png" {
		image, err = png.Decode(body)
	} else if contentType == "image/webp" {
		image, err = webp.Decode(body)
	} else {
		return nil, fmt.Errorf("Unknown content type '%s'", contentType)
	}

	if err != nil {
		return nil, err
	}

	return image, nil
}

func parseManipulators(c echo.Context) []*manipulatorAction {
	// Split / different manipulators
	// Split : manipulator name + params
	// Split , manipulator params
	// Split = manipulator param name + value
	//
	// Example:
	// rotate:a=45,p=5|35/resize:w=405,h=32/output:f=jpg,q=45
	var result []*manipulatorAction
	var err error
	var p = c.Param("*")
	if p, err = url.QueryUnescape(p); err != nil {
		return nil
	}
	var manipulatorsString = strings.Split(p, "/")

	result = make([]*manipulatorAction, len(manipulatorsString))
	for k, v := range manipulatorsString {
		parts := strings.Split(v, ":")
		var manipulatorName string
		var manipulatorParamsString string
		if len(parts) > 0 {
			manipulatorName = parts[0]
		}

		c.Logger().Debugf("Manipulator Name: %s", manipulatorName)

		if len(parts) > 1 {
			manipulatorParamsString = parts[1]
		}

		var manipulatorParams = map[string]string{}
		c.Logger().Debugf("manipulatorParamsString=%s", manipulatorParamsString)
		for _, v := range strings.Split(manipulatorParamsString, ",") {
			var manipulatorParamName string
			var manipulatorParamValue string

			manipulatorParamsParts := strings.Split(v, "=")
			if len(manipulatorParamsParts) > 0 {
				manipulatorParamName = manipulatorParamsParts[0]
			}

			if len(manipulatorParamsParts) > 1 {
				manipulatorParamValue = manipulatorParamsParts[1]
			}

			c.Logger().Debugf("%s=%s", manipulatorParamName, manipulatorParamValue)

			if manipulatorParamName != "" {
				manipulatorParams[manipulatorParamName] = manipulatorParamValue
			}
		}

		if manipulatorName != "" {
			result[k] = &manipulatorAction{manipulatorName, manipulatorParams}
		}
	}

	return result
}

func writeImageToResponse(c echo.Context, contentType string, img image.Image) error {

	if contentType == "image/jpg" || contentType == "image/jpeg" {
		jpeg.Encode(c.Response().Writer, img, &jpeg.Options{Quality: 90})
	} else if contentType == "image/png" {
		png.Encode(c.Response().Writer, img)
	} else {
		return fmt.Errorf("write image to response failed. Unknown content type '%s'", contentType)
	}

	return nil
}

// HandleImage is the image handler
func HandleImage(c echo.Context) error {
	var imageURL string
	var err error
	imageURL, err = url.QueryUnescape(c.Param("url"))
	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid URL passed. Have you tried URL escaping it?")
	}
	c.Logger().Debugf("url=%s", imageURL)

	var parsedURL *url.URL
	if parsedURL, err = url.Parse(imageURL); err != nil {
		return c.String(http.StatusBadRequest, "Unable to parse passed URL")
	}

	fetcher := fetchers.GetFetcherByProtcool(parsedURL.Scheme)
	if fetcher == nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("Cannot handle URLs with scheme \"%s\". url=%s", parsedURL.Scheme, imageURL))
	}

	var imageBody io.Reader
	var contentType string
	if imageBody, contentType, err = fetcher.Fetch(c, imageURL); err != nil {
		c.Logger().Errorf("Failed to fetch image. Reason=%v   url=%s", err, imageURL)
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to fetch image. url=%s", imageURL))
	}

	if imageBody == nil {
		return c.String(http.StatusNotFound, fmt.Sprintf("File not found"))
	}

	c.Logger().Debugf("Image Content-Type=%s   url=%s", contentType, imageURL)

	var img image.Image
	if img, err = loadImage(c, imageURL, contentType, imageBody); err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to load fetched image. url=%s", imageURL))
	}

	m := parseManipulators(c)

	for _, action := range m {
		manipulator := manipulators.GetManipulatorByName(action.Name)
		if manipulator != nil {
			c.Logger().Debugf("Executing Manipulator - %s", action.Name)
			if img, err = manipulator.Execute(c, action.Params, img); err != nil {
				return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to execute manipulator '%s'. Reason: %v", action.Name, err))
			}
		}
	}

	outputContentType := c.Response().Header().Get("Content-Type")
	if outputContentType == "" {
		outputContentType = contentType
		c.Response().Header().Set("Content-Type", outputContentType)
	}

	err = writeImageToResponse(c, outputContentType, img)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to write response")
	}

	c.Response().Status = http.StatusOK
	return nil
}
