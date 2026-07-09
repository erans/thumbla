package manipulators

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"

	_ "golang.org/x/image/webp"

	"github.com/erans/thumbla/config"
	"github.com/erans/thumbla/httpclient"
	"github.com/gofiber/fiber/v2"
)

// CropManipulator crops the image
type PasteManipulator struct {
}

var alignmentRegistry = map[string]func(b image.Rectangle, bb image.Rectangle) image.Rectangle{
	"topcenter": func(b image.Rectangle, bb image.Rectangle) image.Rectangle {
		minX := (b.Max.X / 2) - (bb.Max.X / 2)
		minY := 0
		maxX := minX + bb.Max.X
		maxY := minY + bb.Max.Y
		return image.Rect(minX, minY, maxX, maxY)
	},
	"center": func(b image.Rectangle, bb image.Rectangle) image.Rectangle {
		minX := (b.Max.X / 2) - (bb.Max.X / 2)
		minY := (b.Max.Y / 2) - (bb.Max.Y / 2)
		maxX := minX + bb.Max.X
		maxY := minY + bb.Max.Y
		return image.Rect(minX, minY, maxX, maxY)
	},
	"bottomcenter": func(b image.Rectangle, bb image.Rectangle) image.Rectangle {
		minX := (b.Max.X / 2) - (bb.Max.X / 2)
		minY := b.Max.Y - bb.Max.Y
		maxX := minX + bb.Max.X
		maxY := b.Max.Y
		return image.Rect(minX, minY, maxX, maxY)
	},
	"centerleft": func(b image.Rectangle, bb image.Rectangle) image.Rectangle {
		minX := 0
		minY := (b.Max.Y / 2) - (bb.Max.Y / 2)
		maxX := minX + bb.Max.X
		maxY := minY + bb.Max.Y
		return image.Rect(minX, minY, maxX, maxY)
	},
	"centerright": func(b image.Rectangle, bb image.Rectangle) image.Rectangle {
		minX := b.Max.X - bb.Max.X
		minY := (b.Max.Y / 2) - (bb.Max.Y / 2)
		maxX := b.Max.X
		maxY := minY + bb.Max.Y
		return image.Rect(minX, minY, maxX, maxY)
	},
}

func decodePastedImage(body io.Reader) (image.Image, error) {
	maxSize := config.GetConfig().GetMaxImageSizeBytes()
	data, err := io.ReadAll(io.LimitReader(body, maxSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSize {
		return nil, fmt.Errorf("paste image size (%d bytes) exceeds maximum allowed size (%d bytes)", len(data), maxSize)
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	maxDim := config.GetConfig().GetMaxImageDimension()
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxDim || cfg.Height > maxDim {
		return nil, fmt.Errorf("paste image dimensions (%dx%d) exceed maximum allowed size (%dx%d)",
			cfg.Width, cfg.Height, maxDim, maxDim)
	}

	pastedImg, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return pastedImg, nil
}

// Execute runs the paste manipulator
func (manipulator *PasteManipulator) Execute(c *fiber.Ctx, params map[string]string, img image.Image) (image.Image, error) {
	if imgUrl, ok := params["img"]; ok {
		client := httpclient.NewClient(httpclient.Options{
			TimeoutSeconds: config.GetConfig().GetHTTPTimeout(),
			MaxBytes:       config.GetConfig().GetMaxImageSizeBytes(),
		})
		response, err := client.Get(imgUrl)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()

		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, fmt.Errorf("paste: fetch returned HTTP %d", response.StatusCode)
		}

		pastedImg, err := decodePastedImage(response.Body)
		if err != nil {
			return nil, err
		}

		b := img.Bounds()
		originalImg := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(originalImg, originalImg.Bounds(), img, b.Min, draw.Src)

		bb := pastedImg.Bounds()
		alignment := image.Rect(0, 0, bb.Max.X, bb.Max.Y)

		if align, ok := params["align"]; ok {
			if f, ok := alignmentRegistry[align]; ok {
				alignment = f(b, bb)
			}
		}

		draw.Draw(originalImg, alignment, pastedImg, image.Pt(0, 0), draw.Over)

		return originalImg, nil
	}

	return img, nil
}

func (manipulator *PasteManipulator) Validate(params map[string]string) error { return nil }

// NewPasteManipulator returns a new Paste Manipulator
func NewPasteManipulator(cfg *config.Config) *PasteManipulator {
	return &PasteManipulator{}
}
