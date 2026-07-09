package manipulators

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/erans/thumbla/config"
	"github.com/gofiber/fiber/v2"
)

func TestPasteManipulator_BlocksInternalURL(t *testing.T) {
	config.SetConfig(&config.Config{})
	cfg := &config.Config{}
	config.SetConfig(cfg)
	m := NewPasteManipulator(cfg)

	base := image.NewRGBA(image.Rect(0, 0, 10, 10))
	base.Set(0, 0, color.RGBA{255, 0, 0, 255})

	var c *fiber.Ctx
	_, err := m.Execute(c, map[string]string{"img": "http://127.0.0.1:6379/x"}, base)
	if err == nil {
		t.Fatal("expected SSRF error for internal paste img URL, got none")
	}
	if !strings.Contains(err.Error(), "private/local") && !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected SSRF guard error, got: %v", err)
	}
}
