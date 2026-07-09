package fetchers

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/erans/thumbla/config"
	"github.com/erans/thumbla/httpclient"
	"github.com/erans/thumbla/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/klauspost/compress/zstd"
)

// HTTPFetcher fetches content from http/https sources
type HTTPFetcher struct {
	Name        string
	FetcherType string

	UserName string
	Password string

	Secure                bool
	RestrictHosts         []string
	RestrictPaths         []string
	DisableSSRFProtection bool
}

// Fetch returns content from http/https sources
func (fetcher *HTTPFetcher) Fetch(c *fiber.Ctx, fetchURL string) (io.Reader, string, error) {
	var request *http.Request
	var response *http.Response
	var err error

	// Build shared SSRF-safe options once so the initial URL and redirect targets
	// are validated with identical rules.
	clientOpts := httpclient.Options{
		TimeoutSeconds:        config.GetConfig().GetHTTPTimeout(),
		MaxBytes:              config.GetConfig().GetMaxImageSizeBytes(),
		RestrictHosts:         fetcher.RestrictHosts,
		RestrictPaths:         fetcher.RestrictPaths,
		DisableSSRFProtection: fetcher.DisableSSRFProtection,
	}
	client := httpclient.NewClient(clientOpts)

	// Validate the *initial* URL up front. CheckRedirect only runs on redirects;
	// DialContext only sees host:port and cannot enforce RestrictHosts or
	// RestrictPaths for the first request.
	parsedURL, err := url.Parse(fetchURL)
	if err != nil {
		return nil, "", fmt.Errorf("invalid URL: %w", err)
	}
	if err := httpclient.ValidateURL(parsedURL, clientOpts); err != nil {
		return nil, "", err
	}

	// Create context with timeout for better resource management
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(config.GetConfig().GetHTTPTimeout())*time.Second)
	defer cancel() // Ensure context is cancelled to prevent resource leaks

	if request, err = http.NewRequestWithContext(ctx, "GET", fetchURL, nil); err != nil {
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

	// Check HTTP status code
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, "", fmt.Errorf("HTTP error: %d %s", response.StatusCode, response.Status)
	}

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

	maxSize := config.GetConfig().GetMaxImageSizeBytes()

	// Check content length before reading to prevent memory exhaustion
	if response.ContentLength > 0 {
		if response.ContentLength > maxSize {
			return nil, "", fmt.Errorf("image size (%d bytes) exceeds maximum allowed size (%d bytes)",
				response.ContentLength, maxSize)
		}
	}

	var buf []byte
	if buf, err = io.ReadAll(io.LimitReader(reader, maxSize+1)); err != nil {
		return nil, "", err
	}

	// Double-check actual size read
	if int64(len(buf)) > maxSize {
		return nil, "", fmt.Errorf("image size (%d bytes) exceeds maximum allowed size (%d bytes)",
			len(buf), maxSize)
	}

	var contentType = response.Header.Get("Content-Type")
	log.Printf("Fetched %s Content-Type=%s", fetchURL, contentType)

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
	if hostsInterface, ok := cfg["restrictHosts"]; ok {
		if hostsList, ok := hostsInterface.([]interface{}); ok {
			for _, host := range hostsList {
				if hostStr, ok := host.(string); ok {
					restrictHosts = append(restrictHosts, hostStr)
				}
			}
		} else if hostsList, ok := hostsInterface.([]string); ok {
			restrictHosts = hostsList
		}
	}

	var restrictPaths []string
	if pathsInterface, ok := cfg["restrictPaths"]; ok {
		if pathsList, ok := pathsInterface.([]interface{}); ok {
			for _, path := range pathsList {
				if pathStr, ok := path.(string); ok {
					restrictPaths = append(restrictPaths, pathStr)
				}
			}
		} else if pathsList, ok := pathsInterface.([]string); ok {
			restrictPaths = pathsList
		}
	}

	var disableSSRFProtection bool
	if disableSSRFInterface, ok := cfg["disableSSRFProtection"]; ok {
		if disableSSRF, ok := disableSSRFInterface.(bool); ok {
			disableSSRFProtection = disableSSRF
		}
	}

	return &HTTPFetcher{
		Name:                  utils.SafeCastToString(name),
		FetcherType:           "http",
		UserName:              utils.SafeCastToString(username),
		Password:              utils.SafeCastToString(password),
		Secure:                secure,
		RestrictHosts:         restrictHosts,
		RestrictPaths:         restrictPaths,
		DisableSSRFProtection: disableSSRFProtection,
	}
}
