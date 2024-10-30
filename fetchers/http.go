package fetchers

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"

	"github.com/andybalholm/brotli"
	"github.com/erans/thumbla/utils"
	"github.com/klauspost/compress/zstd"
	"github.com/labstack/echo/v4"
)

// HTTPFetcher fetches content from http/https sources
type HTTPFetcher struct {
	Name        string
	FetcherType string

	UserName string
	Password string

	Secure        bool
	RestrictHosts []string
	RestrictPaths []string
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
	request.Header.Add("Accept-Encoding", "gzip, compress, br, zstd")

	// Add basic auth if username and password are configured
	if fetcher.UserName != "" || fetcher.Password != "" {
		request.SetBasicAuth(fetcher.UserName, fetcher.Password)
	}

	if response, err = client.Do(request); err != nil {
		return nil, "", err
	}

	defer response.Body.Close()

	var reader io.Reader
	switch response.Header.Get("Content-Encoding") {
	case "gzip":
		gzReader, err := gzip.NewReader(response.Body)
		if err != nil {
			return nil, "", err
		}
		defer gzReader.Close()
		reader = gzReader
	case "br":
		reader = brotli.NewReader(response.Body)
		// No need for Close() for brotli reader
	case "zstd":
		zstdReader, err := zstd.NewReader(response.Body)
		if err != nil {
			return nil, "", err
		}
		defer zstdReader.Close()
		reader = zstdReader
	case "deflate":
		reader = flate.NewReader(response.Body)
		defer reader.(io.ReadCloser).Close()
	default:
		reader = response.Body
	}

	var buf []byte
	if buf, err = io.ReadAll(reader); err != nil {
		return nil, "", err
	}

	var contentType = response.Header.Get("Content-Type")
	c.Logger().Debugf("Fetched %s Content-Type=%s", url, contentType)

	return bytes.NewReader(buf), contentType, nil
}

// GetName returns the name assigned to this fetcher that can be used in the 'paths' section
func (fetcher *HTTPFetcher) GetName() string {
	return fetcher.Name
}

// GetFetcherType returns the type of this fetcher to be used in the 'type' properties when defining fetchers
func (fetcher *HTTPFetcher) GetFetcherType() string {
	return fetcher.FetcherType
}

// NewHTTPFetcher creates a new fetcher that support http/https
func NewHTTPFetcher(cfg map[string]interface{}) *HTTPFetcher {
	var ok bool

	var name, _ = cfg["name"]
	var username, _ = cfg["username"]
	var password, _ = cfg["password"]
	var secure bool
	if secure, ok = cfg["secure"].(bool); !ok {
		secure = false
	}
	var restrictHosts []string
	if restrictHosts, ok = cfg["restrictHosts"].([]string); !ok {
		restrictHosts = []string{}
	}

	var restrictPaths []string
	if restrictPaths, ok = cfg["restrictPaths"].([]string); !ok {
		restrictPaths = []string{}
	}

	return &HTTPFetcher{
		Name:          utils.SafeCastToString(name),
		FetcherType:   "http",
		UserName:      utils.SafeCastToString(username),
		Password:      utils.SafeCastToString(password),
		Secure:        secure,
		RestrictHosts: restrictHosts,
		RestrictPaths: restrictPaths,
	}
}
