package fetchers

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/erans/thumbla/config"
	"github.com/erans/thumbla/middleware"
	"github.com/erans/thumbla/utils"
	"github.com/gofiber/fiber/v2"
)

// LocalFetcher fetches content from http/https sources
type LocalFetcher struct {
	Name        string
	FetcherType string

	Path string
}

// Fetch returns content from the local machine
func (fetcher *LocalFetcher) Fetch(c *fiber.Ctx, url string) (io.Reader, string, error) {
	filename := strings.Replace(url, "local://", "", -1)

	// Clean the filename and validate against path traversal
	cleanFilename := filepath.Clean(filename)
	if strings.Contains(cleanFilename, "..") {
		return nil, "", fmt.Errorf("path traversal attempt detected: %s", filename)
	}

	fileFullPath := path.Join(fetcher.Path, cleanFilename)

	// Double-check that the resolved path is still within the allowed directory
	absBasePath, err := filepath.Abs(fetcher.Path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve base path: %w", err)
	}

	absFilePath, err := filepath.Abs(fileFullPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve file path: %w", err)
	}

	if !strings.HasPrefix(absFilePath, absBasePath) {
		return nil, "", fmt.Errorf("path traversal attempt detected: %s", filename)
	}

	if c != nil {
		logger := middleware.GetLoggerFromContext(c)
		logger.Debug().Str("file", fileFullPath).Msg("Loading local file")
	}

	// Check file size before reading to prevent memory exhaustion
	fileInfo, err := os.Stat(fileFullPath)
	if err != nil {
		return nil, "", err
	}

	maxSize := config.GetConfig().GetMaxImageSizeBytes()
	if fileInfo.Size() > maxSize {
		return nil, "", fmt.Errorf("file size (%d bytes) exceeds maximum allowed size (%d bytes)",
			fileInfo.Size(), maxSize)
	}

	var buf []byte
	buf, err = os.ReadFile(fileFullPath)
	if err != nil {
		return nil, "", err
	}

	contentType := utils.GetMimeTypeByFileExt(url)
	if c != nil {
		logger := middleware.GetLoggerFromContext(c)
		logger.Debug().Str("url", url).Str("contentType", contentType).Msg("Determined content type")
	}

	return bytes.NewReader(buf), contentType, nil
}

// GetName returns the name assigned to this fetcher that can be used in the 'paths' section
func (fetcher *LocalFetcher) GetName() string {
	return fetcher.Name
}

// GetFetcherType returns the type of this fetcher to be used in the 'type' properties when defining fetchers
func (fetcher *LocalFetcher) GetFetcherType() string {
	return fetcher.FetcherType
}

// NewLocalFetcher creates a new fetcher that support http/https
func NewLocalFetcher(cfg map[string]interface{}) *LocalFetcher {
	var name, _ = cfg["name"]
	var path, _ = cfg["path"]
	return &LocalFetcher{
		Name:        utils.SafeCastToString(name),
		FetcherType: "local",
		Path:        utils.SafeCastToString(path),
	}
}
