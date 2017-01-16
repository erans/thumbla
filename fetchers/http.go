package fetchers

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/erans/thumbla/config"
	"github.com/labstack/echo"
)

// HTTPFetcher fetches content from http/https sources
type HTTPFetcher struct {
}

// Fetch returns content from http/https sources
func (fetcher *HTTPFetcher) Fetch(c echo.Context, url string) (io.Reader, string, error) {
	var request *http.Request
	var response *http.Response
	var err error

	client := new(http.Client)

	if request, err = http.NewRequest("GET", url, nil); err != nil {
		return nil, "", err
	}
	request.Header.Add("Accept-Encoding", "gzip")

	if response, err = client.Do(request); err != nil {
		return nil, "", err
	}

	defer response.Body.Close()

	var reader io.ReadCloser
	switch response.Header.Get("Content-Encoding") {
	case "gzip":
		if reader, err = gzip.NewReader(response.Body); err != nil {
			return nil, "", err
		}
		defer reader.Close()
	default:
		reader = response.Body
	}

	var buf []byte
	if buf, err = ioutil.ReadAll(response.Body); err != nil {
		return nil, "", err
	}

	var contentType = response.Header.Get("Content-Type")
	c.Logger().Debugf("Fetched %s Content-Type=%s", url, contentType)

	return bytes.NewReader(buf), contentType, nil
}

// NewHTTPFetcher creates a new fetcher that support http/https
func NewHTTPFetcher(cfg *config.Config) *HTTPFetcher {
	return &HTTPFetcher{}
}
