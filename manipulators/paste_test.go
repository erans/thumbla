package manipulators

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/erans/thumbla/config"
	"github.com/gofiber/fiber/v2"
)

func pngConfigOnly(t *testing.T, width, height uint32) []byte {
	t.Helper()

	writeChunk := func(buf *bytes.Buffer, chunkType string, data []byte) {
		t.Helper()
		if err := binary.Write(buf, binary.BigEndian, uint32(len(data))); err != nil {
			t.Fatalf("write chunk length: %v", err)
		}
		buf.WriteString(chunkType)
		buf.Write(data)
		crc := crc32.NewIEEE()
		_, _ = crc.Write([]byte(chunkType))
		_, _ = crc.Write(data)
		if err := binary.Write(buf, binary.BigEndian, crc.Sum32()); err != nil {
			t.Fatalf("write chunk crc: %v", err)
		}
	}

	var buf bytes.Buffer
	buf.Write([]byte{137, 80, 78, 71, 13, 10, 26, 10})
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:4], width)
	binary.BigEndian.PutUint32(ihdr[4:8], height)
	ihdr[8] = 8 // bit depth
	ihdr[9] = 6 // truecolor with alpha
	writeChunk(&buf, "IHDR", ihdr)
	writeChunk(&buf, "IEND", nil)
	return buf.Bytes()
}

func TestDecodePastedImageRejectsOversizedDimensionsBeforeFullDecode(t *testing.T) {
	config.SetConfig(&config.Config{Server: config.ServerConfig{MaxImageDimension: 1, MaxImageSizeBytes: 1024}})

	_, err := decodePastedImage(bytes.NewReader(pngConfigOnly(t, 2, 2)))
	if err == nil {
		t.Fatal("expected dimension error for oversized pasted image, got nil")
	}
	if !strings.Contains(err.Error(), "dimensions") {
		t.Fatalf("expected dimension error, got: %v", err)
	}
}

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
