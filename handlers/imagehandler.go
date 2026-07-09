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

// checkImageDimensions reads format config from body and rejects images
// exceeding maxImageDimension before the full pixel buffer is allocated.
// DecodeConfig consumes bytes, so this function reads the already-size-limited
// body into memory once and returns a fresh reader over the full data.
func checkImageDimensions(contentType string, body io.Reader) (io.Reader, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	maxDim := config.GetConfig().GetMaxImageDimension()
	configReader := bytes.NewReader(data)

	var w, h int
	var cfgErr error
	switch contentType {
	case "image/jpeg", "image/jpg":
		var c image.Config
		c, cfgErr = jpeg.DecodeConfig(configReader)
		w, h = c.Width, c.Height
	case "image/png":
		var c image.Config
		c, cfgErr = png.DecodeConfig(configReader)
		w, h = c.Width, c.Height
	case "image/webp":
		var c image.Config
		c, cfgErr = webp.DecodeConfig(configReader)
		w, h = c.Width, c.Height
	case "image/gif":
		var c image.Config
		c, cfgErr = gif.DecodeConfig(configReader)
		w, h = c.Width, c.Height
	default:
		return bytes.NewReader(data), nil
	}
	if cfgErr != nil {
		return nil, fmt.Errorf("failed to read image config: %w", cfgErr)
	}
	if w > maxDim || h > maxDim || w <= 0 || h <= 0 {
		return nil, fmt.Errorf("image dimensions (%dx%d) exceed maximum allowed size (%dx%d)",
			w, h, maxDim, maxDim)
	}
	return bytes.NewReader(data), nil
}

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

	if contentType != "image/svg+xml" {
		body, err = checkImageDimensions(contentType, body)
		if err != nil {
			return nil, err
		}
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
		if err != nil {
			return nil, fmt.Errorf("failed to parse SVG: %w", err)
		}
		w := int(svgImg.ViewBox.W)
		h := int(svgImg.ViewBox.H)

		maxDim := config.GetConfig().GetMaxImageDimension()
		if w <= 0 || h <= 0 {
			return nil, fmt.Errorf("invalid SVG dimensions: %dx%d", w, h)
		}
		if w > maxDim {
			w = maxDim
		}
		if h > maxDim {
			h = maxDim
		}

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

		if w <= 0 || h <= 0 {
			return nil, fmt.Errorf("invalid SVG target dimensions: %dx%d", w, h)
		}
		if w > maxDim || h > maxDim {
			return nil, fmt.Errorf("SVG target dimensions (%dx%d) exceed maximum allowed size (%dx%d)",
				w, h, maxDim, maxDim)
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

func parseManipulators(c *fiber.Ctx) ([]*manipulatorAction, error) {
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
		return nil, nil
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
					return nil, fmt.Errorf("invalid URL-escaped value for parameter %s: %w", manipulatorParamName, err)
				}
			}

			logger.Debug().Str("param", manipulatorParamName).Str("value", manipulatorParamValue).Msg("Parsed parameter")

			if manipulatorParamName != "" {
				if err := manipulators.ValidateParamShape(manipulatorParamName, manipulatorParamValue); err != nil {
					logger.Warn().
						Str("param", manipulatorParamName).
						Str("value", manipulatorParamValue).
						Err(err).
						Msg("Invalid manipulator parameter")
					return nil, err
				}
				manipulatorParams[manipulatorParamName] = manipulatorParamValue
			}
		}

		if manipulatorName != "" {
			m := manipulators.GetManipulatorByName(manipulatorName)
			if m != nil {
				if verr := m.Validate(manipulatorParams); verr != nil {
					logger.Warn().
						Str("manipulator", manipulatorName).
						Err(verr).
						Msg("Invalid manipulator parameters")
					return nil, verr
				}
			}
			result[k] = &manipulatorAction{manipulatorName, manipulatorParams}
		}
	}

	return result, nil
}

func localString(c *fiber.Ctx, key string) string {
	if v, ok := c.Locals(key).(string); ok {
		return v
	}
	return ""
}

func writeImageToResponse(c *fiber.Ctx, contentType string, img image.Image) error {
	if contentType == "image/jpeg" || contentType == "image/jpg" {
		quality := 90
		if q := localString(c, "X-Quality"); q != "" {
			if n, err := strconv.Atoi(q); err == nil {
				quality = n
			}
		}
		enc := localString(c, "X-Encoder")
		if enc == "" {
			enc = "jpeg"
		}
		if enc != "jpeg" {
			return fmt.Errorf("unsupported encoder: %s", enc)
		}
		jpeg.Encode(c.Response().BodyWriter(), img, &jpeg.Options{Quality: quality})
	} else if contentType == "image/png" {
		png.Encode(c.Response().BodyWriter(), img)
	} else if contentType == "image/webp" {
		quality := 100.0
		if q := localString(c, "X-Quality"); q != "" {
			if f, err := strconv.ParseFloat(q, 32); err == nil {
				quality = f
			}
		}
		options, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, float32(quality))
		if err != nil {
			return fmt.Errorf("failed to create WebP encoder options: %w", err)
		}
		if temp := localString(c, "X-Lossless"); temp == "1" || temp == "true" {
			options.Lossless = true
		}
		if temp := localString(c, "X-Exact"); temp == "1" || temp == "true" {
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

	rawRequestPath := c.Route().Path
	path := rawRequestPath[0:strings.Index(rawRequestPath, "/:url")]
	pathConfig := config.GetConfig().GetPathConfigByPath(path)

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

	var imageBody io.Reader
	var contentType string

	var alternateWidth int = -1
	var alternateHeight int = -1
	var params []string
	var useFetchers = true

	imageURL, params = getFileParams(imageURL)

	if strings.HasSuffix(strings.ToLower(imageURL), ".svg") {
		if len(params) < 2 {
			return c.Status(fiber.StatusBadRequest).SendString("SVG requests require |width,height params")
		}
		alternateWidth, _ = strconv.Atoi(params[0])
		alternateHeight, _ = strconv.Atoi(params[1])
	} else if imageURL == "_blank" {
		if len(params) < 1 || params[0] != "rgba" {
			return c.Status(fiber.StatusBadRequest).SendString("blank image requires |rgba,width,height params")
		}
		if len(params) < 3 {
			return c.Status(fiber.StatusBadRequest).SendString("blank image requires |rgba,width,height params")
		}
		if alternateWidth, err = strconv.Atoi(params[1]); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("blank image width must be an integer")
		}
		if alternateHeight, err = strconv.Atoi(params[2]); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("blank image height must be an integer")
		}
		if alternateWidth <= 0 || alternateHeight <= 0 {
			return c.Status(fiber.StatusBadRequest).SendString("blank image width and height must be positive")
		}
		maxDim := config.GetConfig().GetMaxImageDimension()
		if alternateWidth > maxDim || alternateHeight > maxDim {
			return c.Status(fiber.StatusBadRequest).SendString("blank image dimensions exceed maximum allowed size")
		}

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

	if closer, ok := imageBody.(io.Closer); ok {
		defer closer.Close()
	}

	if imageBody == nil {
		return c.Status(fiber.StatusNotFound).SendString("file not found")
	}

	logger.Debug().Str("contentType", contentType).Str("imageURL", imageURL).Msg("Image fetched successfully")

	var img image.Image
	if img, err = loadImage(c, imageURL, contentType, imageBody, alternateWidth, alternateHeight); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("failed to load fetched image. url=%s", imageURL))
	}

	m, err := parseManipulators(c)
	if err != nil {
		logger.Warn().Err(err).Msg("Invalid manipulator parameters")
		return c.Status(fiber.StatusBadRequest).SendString("Invalid manipulator parameters")
	}

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
