package fetchers

import (
	"io"

	"github.com/erans/thumbla/config"
	"github.com/gofiber/fiber/v2"
)

// Fetcher interface handles fetching content from different sources
type Fetcher interface {
	GetName() string
	GetFetcherType() string
	Fetch(ctx *fiber.Ctx, url string) (responseBody io.Reader, contentType string, err error)
}

var fetcherRegistry []Fetcher
var fetcherByType = map[string]Fetcher{}
var fetcherByName = map[string]Fetcher{}
var fetcherByPath = map[string]Fetcher{}

// InitFetchers initializes fetchers from config
func InitFetchers(cfg *config.Config) {
	fetcherRegistry = make([]Fetcher, len(cfg.Fetchers))
	for i, fetcherCfgData := range cfg.Fetchers {
		if fetcherType, ok := fetcherCfgData["type"]; ok {
			var fetcher Fetcher
			switch fetcherType {
			case "local":
				fetcher = NewLocalFetcher(fetcherCfgData)
			case "http":
				fetcher = NewHTTPFetcher(fetcherCfgData)
			case "s3":
				fetcher = NewS3Fetcher(fetcherCfgData)
			case "gs":
				fetcher = NewGoogleStroageFetcher(fetcherCfgData)
			case "ds":
				fetcher = NewDigitalOceanSpacesFetcher(fetcherCfgData)
			case "r2":
				fetcher = NewCloudflareR2Fetcher(fetcherCfgData)
			}

			fetcherRegistry[i] = fetcher
		}
	}

	for _, fetcher := range fetcherRegistry {
		fetcherByName[fetcher.GetName()] = fetcher
		fetcherByType[fetcher.GetFetcherType()] = fetcher
	}

	for _, p := range cfg.Paths {
		if fetcher, ok := fetcherByName[p.FetcherName]; ok {
			fetcherByPath[p.Path] = fetcher
		}
	}
}

// GetFetcherByName returns a fetcher by its configured name
func GetFetcherByName(name string) Fetcher {
	if fetcherByName != nil {
		if fetcher, ok := fetcherByName[name]; ok {
			return fetcher
		}
	}

	return nil
}

// GetFetcherByType returns the fetcher specified by its type
func GetFetcherByType(fetcherType string) Fetcher {
	if fetcherByType != nil {
		if fetcher, ok := fetcherByType[fetcherType]; ok {
			return fetcher
		}
	}

	return nil
}

// GetFetcherByPath returns the fetcher specified by its path
func GetFetcherByPath(path string) Fetcher {
	if fetcherByPath != nil {
		if fetcher, ok := fetcherByPath[path]; ok {
			return fetcher
		}
	}

	return nil
}
