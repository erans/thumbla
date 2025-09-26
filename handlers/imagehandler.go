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
	"net/url"
	"strconv"
	"strings"

	"github.com/kolesa-team/go-webp/encoder"
	kolesawebp "github.com/kolesa-team/go-webp/webp"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/webp"

	"github.com/gofiber/fiber/v2"

	"github.com/erans/thumbla/config"
	"github.com/erans/thumbla/fetchers"
	"github.com/erans/thumbla/manipulators"
	"github.com/erans/thumbla/middleware"
	"github.com/erans/thumbla/utils"
)

type manipulatorAction struct {
	Name   string
	Params map[string]string
}

func loadImage(c *fiber.Ctx, url string, contentType string, body io.Reader, alternativeWidth int, alternativeHeight int) (image.Image, error) {
	var img image.Image
	var err error

	logger := middleware.GetLoggerFromContext(c)
	logger.Debug().Str("contentType", contentType).Msg("Loading image")

	if contentType == "" {
		contentType = utils.GetMimeTypeByFileExt(url)
	}

	if contentType == "" {
		return nil, fmt.Errorf("content Type is missing and could not be inferred")
	}

	if contentType == "image/jpeg" || contentType == "image/jpg" {
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

	// Validate image dimensions to prevent memory exhaustion attacks
	if img != nil {
		bounds := img.Bounds()
		width := bounds.Dx()
		height := bounds.Dy()

		cfg := config.GetConfig()
		maxDimension := cfg.GetMaxImageDimension()

		if width > maxDimension || height > maxDimension {
			logger.Warn().
				Int("width", width).
				Int("height", height).
				Int("maxDimension", maxDimension).
				Msg("Image dimensions exceed maximum allowed size")
			return nil, fmt.Errorf("image dimensions (%dx%d) exceed maximum allowed size (%dx%d)",
				width, height, maxDimension, maxDimension)
		}

		logger.Debug().
			Int("width", width).
			Int("height", height).
			Msg("Image dimensions validated")
	}

	return img, nil
}

// validateManipulatorParameter validates manipulator parameter values
func validateManipulatorParameter(paramName, paramValue string) error {
	// Validate parameter name is not too long (prevent memory exhaustion)
	if len(paramName) > 50 {
		return fmt.Errorf("parameter name too long: %d characters", len(paramName))
	}

	// Validate parameter value is not too long
	if len(paramValue) > 100 {
		return fmt.Errorf("parameter value too long: %d characters", len(paramValue))
	}

	// Check for numeric parameters and validate ranges
	if paramValue != "" {
		// Common numeric parameters that should have reasonable bounds
		numericParams := map[string]struct {
			min, max float64
		}{
			"w":       {1, 20000},      // width: 1px to 20,000px
			"h":       {1, 20000},      // height: 1px to 20,000px
			"q":       {1, 100},        // quality: 1% to 100%
			"a":       {-360, 360},     // angle: -360° to 360°
			"x":       {0, 20000},      // x coordinate: 0 to 20,000px
			"y":       {0, 20000},      // y coordinate: 0 to 20,000px
			"r":       {0, 255},        // RGB values: 0 to 255
			"g":       {0, 255},        // RGB values: 0 to 255
			"b":       {0, 255},        // RGB values: 0 to 255
			"a_color": {0, 255},        // Alpha: 0 to 255
		}

		if bounds, isNumeric := numericParams[paramName]; isNumeric {
			if val, err := strconv.ParseFloat(paramValue, 64); err == nil {
				if val < bounds.min || val > bounds.max {
					return fmt.Errorf("parameter %s value %g is outside valid range [%g, %g]",
						paramName, val, bounds.min, bounds.max)
				}
			} else {
				return fmt.Errorf("parameter %s requires numeric value, got: %s", paramName, paramValue)
			}
		}

		// Validate format parameter has only allowed values
		if paramName == "f" {
			allowedFormats := map[string]bool{
				"jpg":  true,
				"jpeg": true,
				"png":  true,
				"webp": true,
				"gif":  true,
			}
			if !allowedFormats[strings.ToLower(paramValue)] {
				return fmt.Errorf("unsupported format: %s", paramValue)
			}
		}

		// Validate boolean parameters
		if paramName == "lossless" || paramName == "progressive" {
			if paramValue != "true" && paramValue != "false" && paramValue != "1" && paramValue != "0" {
				return fmt.Errorf("parameter %s requires boolean value (true/false/1/0), got: %s",
					paramName, paramValue)
			}
		}
	}

	return nil
}

func parseManipulators(c *fiber.Ctx) []*manipulatorAction {
	// Split / different manipulators
	// Split : manipulator name + params
	// Split , manipulator params
	// Split = manipulator param name + value
	//
	// Example:
	// rotate:a=45,p=5|35/resize:w=405,h=32/output:f=jpg,q=45
	var result []*manipulatorAction
	var err error
	var p = c.Params("*")

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

		logger := middleware.GetLoggerFromContext(c)
	logger.Debug().Str("manipulator", manipulatorName).Msg("Processing manipulator")

		if len(parts) > 1 {
			manipulatorParamsString = parts[1]
		}

		var manipulatorParams = map[string]string{}
		logger.Debug().Str("params", manipulatorParamsString).Msg("Parsing manipulator parameters")
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

			logger.Debug().Str("param", manipulatorParamName).Str("value", manipulatorParamValue).Msg("Parsed parameter")

			if manipulatorParamName != "" {
				// Validate parameter values to prevent attacks
				if err := validateManipulatorParameter(manipulatorParamName, manipulatorParamValue); err != nil {
					logger.Warn().
						Str("param", manipulatorParamName).
						Str("value", manipulatorParamValue).
						Err(err).
						Msg("Invalid manipulator parameter")
					return nil
				}
				manipulatorParams[manipulatorParamName] = manipulatorParamValue
			}
		}

		if manipulatorName != "" {
			result[k] = &manipulatorAction{manipulatorName, manipulatorParams}
		}
	}

	return result
}

func writeImageToResponse(c *fiber.Ctx, contentType string, img image.Image) error {

	if contentType == "image/jpeg" || contentType == "image/jpg" {
		var quality = 90
		var tempQuality = c.Get("X-Quality")
		if tempQuality != "" {
			quality, _ = strconv.Atoi(tempQuality)
		}

		c.Response().Header.Del("X-Quality")

		var encoder = c.Get("X-Encoder")
		if encoder == "" {
			encoder = "jpeg"
		}

		if encoder == "jpeg" {
			jpeg.Encode(c.Response().BodyWriter(), img, &jpeg.Options{Quality: quality})
		}
	} else if contentType == "image/png" {
		png.Encode(c.Response().BodyWriter(), img)
	} else if contentType == "image/webp" {
		var quality = 100.0
		var tempQuality = c.Get("X-Quality")
		if tempQuality != "" {
			quality, _ = strconv.ParseFloat(tempQuality, 32)
		}
		//options := &chaiwebp.Options{Quality: float32(quality)}

		options, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, float32(quality))
		if err != nil {
			return fmt.Errorf("failed to create WebP encoder options: %w", err)
		}

		var temp = c.Get("X-Lossless")
		if temp != "" && (temp == "1" || temp == "true") {
			options.Lossless = true
		}

		temp = c.Get("X-Exact")
		if temp != "" && (temp == "1" || temp == "true") {
			options.Exact = 1
		}
		kolesawebp.Encode(c.Response().BodyWriter(), img, options)
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
func HandleImage(c *fiber.Ctx) error {
	logger := middleware.GetLoggerFromContext(c)
	logger.Debug().Str("path", c.Path()).Msg("Handling image request")

	pathConfig := config.GetConfig().GetPathConfigByPath(c.Route().Path)

	var imageURL string
	var err error
	imageURL, err = url.QueryUnescape(c.Params("url"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid URL passed. Have you tried URL escaping it?")
	}
	logger.Debug().Str("imageURL", imageURL).Msg("Decoded image URL")

	var parsedURL *url.URL
	if parsedURL, err = url.Parse(imageURL); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Unable to parse passed URL")
	}

	logger.Debug().Str("routePath", c.Route().Path).Msg("Searching for fetcher")
	logger.Debug().Str("parsedURL", parsedURL.String()).Msg("Parsed URL")

	rawRequestPath := c.Route().Path
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
				return c.Status(fiber.StatusBadRequest).SendString("failed to create blank image")
			}

			imageBody = buf
			contentType = "image/png"
			useFetchers = false
		}
	}

	if useFetchers {
		fetcher := fetchers.GetFetcherByPath(path)
		if fetcher == nil {
			logger.Error().Str("path", path).Msg("No fetcher defined for path")
			return c.Status(fiber.StatusBadRequest).SendString("No fetcher is defined for specified path")
		}

		if imageBody, contentType, err = fetcher.Fetch(c, imageURL); err != nil {
			logger.Error().Err(err).Str("imageURL", imageURL).Msg("Failed to fetch image")
			return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Failed to fetch image. url=%s", imageURL))
		}
	}

	if imageBody == nil {
		return c.Status(fiber.StatusNotFound).SendString("file not found")
	}

	logger.Debug().Str("contentType", contentType).Str("imageURL", imageURL).Msg("Image fetched successfully")

	var img image.Image
	if img, err = loadImage(c, imageURL, contentType, imageBody, alternateWidth, alternateHeight); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("failed to load fetched image. url=%s", imageURL))
	}

	m := parseManipulators(c)

	for _, action := range m {
		logger.Debug().Str("manipulator", action.Name).Msg("Applying manipulator")
		manipulator := manipulators.GetManipulatorByName(action.Name)
		if manipulator != nil {
			logger.Debug().Str("manipulator", action.Name).Msg("Executing manipulator")
			if img, err = manipulator.Execute(c, action.Params, img); err != nil {
				return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("failed to execute manipulator '%s'. Reason: %v", action.Name, err))
			}
		}
	}

	outputContentType := c.GetRespHeader("Content-Type")
	if outputContentType == "" {
		outputContentType = contentType
		c.Set("Content-Type", outputContentType)
	}

	var cacheControlHeaderValue = config.GetConfig().CacheControlHeader
	if pathConfig != nil && pathConfig.CacheControl != "" {
		cacheControlHeaderValue = pathConfig.CacheControl
	}
	logger.Debug().Str("cacheControl", cacheControlHeaderValue).Msg("Setting cache control header from config")
	if cacheControlHeaderValue != "" {
		logger.Debug().Str("cacheControl", cacheControlHeaderValue).Msg("Applied cache control header")
		c.Set("Cache-Control", cacheControlHeaderValue)
	}

	err = writeImageToResponse(c, outputContentType, img)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to write response")
	}

	c.Status(fiber.StatusOK)
	return nil
}
