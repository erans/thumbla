package handlers

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/webp"

	//guetzli "github.com/chai2010/guetzli-go"
	"github.com/labstack/echo/v4"

	"github.com/erans/thumbla/config"
	"github.com/erans/thumbla/fetchers"
	"github.com/erans/thumbla/manipulators"
	"github.com/erans/thumbla/utils"
)

type manipulatorAction struct {
	Name   string
	Params map[string]string
}

func loadImage(c echo.Context, url string, contentType string, body io.Reader, alternativeWidth int, alternativeHeight int) (image.Image, error) {
	var img image.Image
	var err error

	c.Logger().Debugf("loadImage contentType=%s", contentType)

	if contentType == "" {
		contentType = utils.GetMimeTypeByFileExt(url)
	}

	if contentType == "" {
		return nil, fmt.Errorf("content Type is missing and could not be inferred")
	}

	if contentType == "image/jpg" || contentType == "image/jpeg" {
		img, err = jpeg.Decode(body)
	} else if contentType == "image/png" {
		img, err = png.Decode(body)
	} else if contentType == "image/webp" {
		img, err = webp.Decode(body)
	} else if contentType == "image/gif" {
		img, err = gif.Decode(body)
	} else if contentType == "image/svg+xml" {
		var svgImg *oksvg.SvgIcon
		svgImg, err = oksvg.ReadIconStream(body)
		w := int(svgImg.ViewBox.W)
		h := int(svgImg.ViewBox.H)

		ratio := math.Max(float64(w), float64(h)) / math.Min(float64(w), float64(h))
		widthBigger := w > h

		if alternativeHeight == -1 && alternativeWidth == -1 {
			// Keep the original width and height
		} else if alternativeHeight == -1 {
			w = alternativeWidth
			if widthBigger {
				h = int(float64(w) / ratio)
			} else {
				h = int(float64(w) * ratio)
			}
		} else if alternativeWidth == -1 {
			h = alternativeHeight
			if widthBigger {
				w = int(float64(h) * ratio)
			} else {
				w = int(float64(h) / ratio)
			}
		} else {
			h = alternativeHeight
			w = alternativeWidth
		}

		svgImg.SetTarget(0, 0, float64(w), float64(h))

		var tempImg *image.RGBA64 = image.NewRGBA64(image.Rect(0, 0, w, h))
		svgImg.Draw(rasterx.NewDasher(w, h, rasterx.NewScannerGV(w, h, tempImg, tempImg.Bounds())), 1)

		img = tempImg
	} else {
		return nil, fmt.Errorf("unknown content type '%s'", contentType)
	}

	if err != nil {
		return nil, err
	}

	return img, nil
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

	// There are no manipulators on the URL
	if p == "" {
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
				if manipulatorParamValue, err = url.QueryUnescape(manipulatorParamValue); err != nil {
					return nil
				}
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
		var quality = 90
		var tempQuality = c.Response().Header().Get("X-Quality")
		if tempQuality != "" {
			quality, _ = strconv.Atoi(tempQuality)
		}

		c.Response().Header().Del("X-Quality")

		var encoder = c.Response().Header().Get("X-Encoder")
		if encoder == "" {
			encoder = "jpeg"
		}

		if encoder == "jpeg" {
			jpeg.Encode(c.Response().Writer, img, &jpeg.Options{Quality: quality})
		} /* else if encoder == "guetzli" {
			c.Logger().Info("Using guetzli")
			guetzli.Encode(c.Response().Writer, img, &guetzli.Options{Quality: quality})
		}*/

	} else if contentType == "image/png" {
		png.Encode(c.Response().Writer, img)
	} else {
		return fmt.Errorf("write image to response failed. Unknown content type '%s'", contentType)
	}

	return nil
}

func getFileParams(imageURL string) (string, []string) {
	if !strings.ContainsAny(imageURL, "|") {
		return imageURL, nil
	}

	parts := strings.Split(imageURL, "|")
	params := strings.Split(parts[1], ",")

	return parts[0], params
}

// HandleImage is the image handler
func HandleImage(c echo.Context) error {
	c.Logger().Debugf("Path: %v", c.Path())

	pathConfig := config.GetConfig().GetPathConfigByPath(c.Path())

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

	c.Logger().Debugf("Searching for fetcher for path: %s", c.Path())
	c.Logger().Debugf("URL given: %s", parsedURL.String())

	rawRequestPath := c.Path()
	path := rawRequestPath[0:strings.Index(rawRequestPath, "/:url")]

	var imageBody io.Reader
	var contentType string

	var alternateWidth int = -1
	var alternateHeight int = -1
	var params []string
	var useFetchers = true

	imageURL, params = getFileParams(imageURL)

	if strings.HasSuffix(strings.ToLower(imageURL), ".svg") {
		alternateWidth, _ = strconv.Atoi(params[0])
		alternateHeight, _ = strconv.Atoi(params[1])
	} else if imageURL == "_blank" {
		if params[0] == "rgba" {
			alternateWidth, _ = strconv.Atoi(params[1])
			alternateHeight, _ = strconv.Atoi(params[2])

			img := image.NewRGBA(image.Rectangle{image.Point{0, 0}, image.Point{alternateWidth, alternateHeight}})
			buf := new(bytes.Buffer)
			err := png.Encode(buf, img)
			if err != nil {
				return c.String(http.StatusBadRequest, "failed to create blank image")
			}

			imageBody = buf
			contentType = "image/png"
			useFetchers = false
		}
	}

	if useFetchers {
		fetcher := fetchers.GetFetcherByPath(path)
		if fetcher == nil {
			c.Logger().Errorf("No fetcher is defined for specified path")
			return c.String(http.StatusBadRequest, "No fetcher is defined for specified path")
		}

		if imageBody, contentType, err = fetcher.Fetch(c, imageURL); err != nil {
			c.Logger().Errorf("Failed to fetch image. Reason=%v   url=%s", err, imageURL)
			return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to fetch image. url=%s", imageURL))
		}
	}

	if imageBody == nil {
		return c.String(http.StatusNotFound, "file not found")
	}

	c.Logger().Debugf("Image Content-Type=%s   url=%s", contentType, imageURL)

	var img image.Image
	if img, err = loadImage(c, imageURL, contentType, imageBody, alternateWidth, alternateHeight); err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("failed to load fetched image. url=%s", imageURL))
	}

	m := parseManipulators(c)

	for _, action := range m {
		c.Logger().Debugf("Manipulator requested: %s", action.Name)
		manipulator := manipulators.GetManipulatorByName(action.Name)
		if manipulator != nil {
			c.Logger().Debugf("Executing Manipulator - %s", action.Name)
			if img, err = manipulator.Execute(c, action.Params, img); err != nil {
				return c.String(http.StatusInternalServerError, fmt.Sprintf("failed to execute manipulator '%s'. Reason: %v", action.Name, err))
			}
		}
	}

	outputContentType := c.Response().Header().Get("Content-Type")
	if outputContentType == "" {
		outputContentType = contentType
		c.Response().Header().Set("Content-Type", outputContentType)
	}

	var cacheControlHeaderValue = config.GetConfig().CacheControlHeader
	if pathConfig != nil && pathConfig.CacheControl != "" {
		cacheControlHeaderValue = pathConfig.CacheControl
	}
	c.Logger().Debugf("Config cache-control header value: %s", cacheControlHeaderValue)
	if cacheControlHeaderValue != "" {
		c.Logger().Debugf("Setting cache-control header value: %s", cacheControlHeaderValue)
		c.Response().Header().Set("Cache-Control", cacheControlHeaderValue)
	}

	err = writeImageToResponse(c, outputContentType, img)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to write response")
	}

	c.Response().Status = http.StatusOK
	return nil
}
