package fetchers

import (
	"io"

	"github.com/erans/thumbla/config"
	"github.com/labstack/echo"
)

// Fetcher interface handles fetching content from different sources
type Fetcher interface {
	Fetch(c echo.Context, url string) (responseBody io.Reader, contentType string, err error)
}

var fetcherRegistry map[string]Fetcher

// InitFetchers initializes fetchers from config
func InitFetchers(cfg *config.Config) {
	fetcherRegistry = map[string]Fetcher{
		"http":  NewHTTPFetcher(cfg),
		"https": NewHTTPFetcher(cfg),
		"local": NewLocalFetcher(cfg),
		"s3":    NewS3Fetcher(cfg),
		"gs":    NewGoogleStroageFetcher(cfg),
	}
}

// GetFetcherByProtcool returns the fetcher specified by protocol
func GetFetcherByProtcool(protocol string) Fetcher {
	if fetcherRegistry != nil {
		if fetcher, ok := fetcherRegistry[protocol]; ok {
			return fetcher
		}
	}

	return nil
}
