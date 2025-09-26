package manipulators

import (
	"image"
	"image/draw"
	"net/http"

	"github.com/erans/thumbla/config"
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

// Execute runs the paste manipulator
func (manipulator *PasteManipulator) Execute(c *fiber.Ctx, params map[string]string, img image.Image) (image.Image, error) {
	if imgUrl, ok := params["img"]; ok {
		response, err := http.Get(imgUrl)
		if err != nil {
			return nil, err
		}

		defer response.Body.Close()
		pastedImg, _, err := image.Decode(response.Body)
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

// NewPasteManipulator returns a new Paste Manipulator
func NewPasteManipulator(cfg *config.Config) *PasteManipulator {
	return &PasteManipulator{}
}
