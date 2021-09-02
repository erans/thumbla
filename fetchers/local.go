package fetchers

import (
	"bytes"
	"io"
	"io/ioutil"
	"path"
	"strings"

	"github.com/erans/thumbla/utils"
	"github.com/labstack/echo/v4"
)

// LocalFetcher fetches content from http/https sources
type LocalFetcher struct {
	Name        string
	FetcherType string

	Path string
}

// Fetch returns content from the local machine
func (fetcher *LocalFetcher) Fetch(c echo.Context, url string) (io.Reader, string, error) {
	filename := strings.Replace(url, "local://", "", -1)
	fileFullPath := path.Join(fetcher.Path, filename)

	c.Logger().Debugf("File to load: %s", fileFullPath)

	var buf []byte
	var err error
	if buf, err = ioutil.ReadFile(fileFullPath); err != nil {
		return nil, "", err
	}

	c.Logger().Debugf("url=%s  GetMimeTypeByFileExt=%s", url, utils.GetMimeTypeByFileExt(url))

	return bytes.NewReader(buf), utils.GetMimeTypeByFileExt(url), nil
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
