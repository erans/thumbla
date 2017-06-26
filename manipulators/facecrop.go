package manipulators

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"net/url"
	"strconv"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/anthonynsimon/bild/transform"
	"github.com/erans/thumbla/cache"
	"github.com/erans/thumbla/config"
	"github.com/erans/thumbla/manipulators/face"
	"github.com/labstack/echo"
)

// FaceCropManipulator crops the image in a smart way to include most of the faces in the image
type FaceCropManipulator struct {
	DefaultProvider string
	Cfg             *config.Config
}

func (m *FaceCropManipulator) drawRect(x1, y1, x2, y2, thickness int, col color.RGBA, img image.RGBA) {
	for t := 0; t < thickness; t++ {
		// draw horizontal lines
		for x := x1; x <= x2; x++ {
			img.Set(x, y1+t, col)
			img.Set(x, y2-t, col)
		}
		// draw vertical lines
		for y := y1; y <= y2; y++ {
			img.Set(x1+t, y, col)
			img.Set(x2-t, y, col)
		}
	}
}

func (m *FaceCropManipulator) drawLabel(img *image.RGBA, x, y int, col color.RGBA, text string) {
	point := fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(y * 64)}

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  point,
	}
	d.DrawString(text)
}

func (m *FaceCropManipulator) addLabel(img *image.RGBA, x, y int, col color.RGBA, text string, drawShadow bool) {
	if drawShadow {
		m.drawLabel(img, x+1, y+1, color.RGBA{0, 0, 0, 255}, text)
	}
	m.drawLabel(img, x, y, col, text)
}

// Execute runs the fit manipulator and fits the image to the specified size
func (m *FaceCropManipulator) Execute(c echo.Context, params map[string]string, img image.Image) (image.Image, error) {
	var debugImage image.RGBA
	var debug = false
	if val, ok := params["debug"]; ok {
		if val == "1" {
			debug = true
			switch i := img.(type) {
			case *image.YCbCr:
				b := img.Bounds()
				m := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
				draw.Draw(m, m.Bounds(), img, b.Min, draw.Src)

				img = m
				debugImage = *m
			case *image.RGBA:
				debugImage = *i
			}
		}
	}

	var provider = m.Cfg.FaceAPI.DefaultProvider

	if p, ok := params["provider"]; ok {
		provider = p
	}

	c.Logger().Debugf("Try to find detection for provider '%s'", provider)
	if detector := face.GetDetectorByName(provider); detector != nil {
		var err error
		var faces []image.Rectangle

		var imageURL, _ = url.QueryUnescape(c.Param("url"))

		var cacheKey = fmt.Sprintf("face-%s-%s", provider, imageURL)

		var useCache = true
		if v, ok := params["useCache"]; ok {
			useCache = v != "0"
		}

		if !useCache || !cache.GetCache().Contains(cacheKey) {
			faces, err = detector.Detect(c, m.Cfg, params, img)
			if err != nil {
				c.Logger().Errorf("%v", err)
			}

			if useCache {
				cache.GetCache().Set(cacheKey, faces)
			}
		} else {
			faces = cache.GetCache().Get(cacheKey).([]image.Rectangle)
			c.Logger().Debugf("Found faces cache")
		}

		c.Logger().Debugf("Faces: %v", faces)
		if faces == nil || len(faces) < 0 {
			return img, nil
		}

		// Find bounding rectangle of all faces
		var minX0 = faces[0].Min.X
		var minY0 = faces[0].Min.Y
		var maxX1 = faces[0].Max.X
		var maxY1 = faces[0].Max.Y

		for i, v := range faces {
			if debug {
				m.drawRect(v.Min.X, v.Min.Y, v.Max.X, v.Max.Y, 3, color.RGBA{0, 0, 255, 255}, debugImage)
				m.addLabel(&debugImage, v.Min.X+10, v.Min.Y+20, color.RGBA{0, 0, 255, 255}, fmt.Sprintf("%dx%d - Face %d", v.Max.X-v.Min.X, v.Max.Y-v.Min.Y, i), true)
			}

			if v.Min.X < minX0 {
				minX0 = v.Min.X
			}

			if v.Min.Y < minY0 {
				minY0 = v.Min.Y
			}

			if v.Max.X > maxX1 {
				maxX1 = v.Max.X
			}

			if v.Max.Y > maxY1 {
				maxY1 = v.Max.Y
			}
		}

		var boundMin = image.Point{X: minX0, Y: minY0}
		var boundMax = image.Point{X: maxX1, Y: maxY1}

		boundWidth := boundMax.X - boundMin.X
		boundHeight := boundMax.Y - boundMin.Y
		width := float64(img.Bounds().Dx())
		height := float64(img.Bounds().Dy())
		imgRatio := math.Max(width, height) / math.Min(width, height)

		c.Logger().Debugf("Bound Min: %v", boundMin)
		c.Logger().Debugf("Bound Max: %v", boundMax)

		if debug {
			// Draw the bounding rectangle before padding
			m.drawRect(boundMin.X, boundMin.Y, boundMax.X, boundMax.Y, 4, color.RGBA{0, 255, 0, 255}, debugImage)
			m.addLabel(&debugImage, boundMin.X+10, boundMin.Y+20, color.RGBA{0, 255, 0, 255}, fmt.Sprintf("%dx%d - Faces Bound Rect", boundWidth, boundHeight), true)
		}

		// Add padding to capture slightly more than the faces
		var padding = 0.2
		if v, ok := params["pp"]; ok {
			padding, _ = strconv.ParseFloat(v, 64)
		}

		c.Logger().Debugf("BoundRectWidth=%d  BoundRectHeight=%d", boundWidth, boundHeight)

		widthPadding := int(float64(boundWidth) * padding)
		heightPadding := int(float64(boundHeight) * padding)

		c.Logger().Debugf("Width Padding=%d  Height Padding=%d", widthPadding, heightPadding)

		boundMin.X -= widthPadding
		boundMin.Y -= heightPadding

		boundMax.X += widthPadding
		boundMax.Y += heightPadding

		boundWidth = boundMax.X - boundMin.X
		boundHeight = boundMax.Y - boundMin.Y

		if debug {
			// Draw the bounding rectangle after padding
			m.drawRect(boundMin.X, boundMin.Y, boundMax.X, boundMax.Y, 4, color.RGBA{255, 255, 0, 255}, debugImage)
			m.addLabel(&debugImage, boundMin.X+10, boundMin.Y+20, color.RGBA{255, 255, 0, 255}, fmt.Sprintf("%dx%d - Faces Bound Rect Padded", boundWidth, boundHeight), true)
		}

		var keepImageOrientation = true

		if v, ok := params["kio"]; ok {
			keepImageOrientation = (v == "1")
		}

		// Keep the face crop with the same image orientation so that it can be used
		// the same way as the original image was used
		if keepImageOrientation {
			if img.Bounds().Dy() > img.Bounds().Dx() && boundWidth > boundHeight {
				boundHeight = int(float64(boundWidth) / imgRatio)
				boundRectCenter := image.Point{X: boundMin.X + boundWidth/2, Y: boundMin.Y + boundHeight/2}
				boundMin.X = boundRectCenter.X - (boundHeight / 2)
				boundMin.Y = boundRectCenter.Y - (boundWidth / 2)
				boundMax.X = boundRectCenter.X + (boundHeight / 2)
				boundMax.Y = boundRectCenter.Y + (boundWidth / 2)

				if boundMin.Y < 0 {
					boundMax.Y = boundMax.Y + (-1 * boundMin.Y)
					boundMin.Y = 0
				}

				if boundMax.Y > int(height) {
					boundMin.Y -= boundMax.Y - int(height)
					boundMax.Y = int(height)
				}
			}
		}

		c.Logger().Debugf("Resized Bound Min %v", boundMin)
		c.Logger().Debugf("Resized Bound Max %v", boundMax)

		boundWidth = boundMax.X - boundMin.X
		boundHeight = boundMax.Y - boundMin.Y

		if debug {
			// Draw the bounding rectangle after padding
			m.drawRect(boundMin.X, boundMin.Y, boundMax.X, boundMax.Y, 4, color.RGBA{255, 0, 0, 255}, debugImage)
			m.addLabel(&debugImage, boundMin.X+10, boundMin.Y+20, color.RGBA{255, 0, 0, 255}, fmt.Sprintf("%dx%d - Final image to be cropped", boundWidth, boundHeight), true)
		}

		if !debug {
			return transform.Crop(img, image.Rect(boundMin.X, boundMin.Y, boundMax.X, boundMax.Y)), nil
		}
	}
	return img, nil
}

// NewFaceCropManipulator returns a new face crop Manipulator
func NewFaceCropManipulator(cfg *config.Config) *FaceCropManipulator {
	return &FaceCropManipulator{DefaultProvider: cfg.FaceAPI.DefaultProvider, Cfg: cfg}
}
