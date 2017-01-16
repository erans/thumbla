package fetchers

import (
	"bytes"
	"io"
	"io/ioutil"
	"path"
	"strings"

	"github.com/erans/thumbla/config"
	"github.com/erans/thumbla/utils"
	"github.com/labstack/echo"
)

// LocalFetcher fetches content from http/https sources
type LocalFetcher struct {
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

// NewLocalFetcher creates a new fetcher that support http/https
func NewLocalFetcher(cfg *config.Config) *LocalFetcher {
	var path = cfg.GetFetcherConfigKeyValue("local", "path")
	return &LocalFetcher{Path: path}
}
