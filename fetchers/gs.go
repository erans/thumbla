package fetchers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"

	"github.com/erans/thumbla/config"
	"github.com/labstack/echo"
)

const (
	// SecuritySourceBackground is used when security is set from the compute instance level
	SecuritySourceBackground = "background"
	// SecuritySourceFile is used when security is set by JWT config from a JSON file
	SecuritySourceFile = "file"
)

// GoogleStroageFetcher fetches content from http/https sources
//
// URL Format used: gs://bucketName/path/file
type GoogleStroageFetcher struct {
	ProjectID              string
	SecuritySource         string
	ServiceAccountJSONFile string
}

func (fetcher *GoogleStroageFetcher) getBucketAndObjectKeyFromURL(c echo.Context, fileURL string) (string, string) {
	if u, err := url.Parse(fileURL); err == nil {
		return u.Host, u.Path
	}

	return "", ""
}

func (fetcher *GoogleStroageFetcher) getClient(ctx context.Context) (*storage.Client, error) {
	var options option.ClientOption

	if fetcher.SecuritySource == SecuritySourceFile {
		options = option.WithServiceAccountFile(fetcher.ServiceAccountJSONFile)
	}

	return storage.NewClient(ctx, options)
}

// Fetch returns content from the local machine
func (fetcher *GoogleStroageFetcher) Fetch(c echo.Context, url string) (io.Reader, string, error) {
	var err error
	var client *storage.Client

	ctx := context.Background()

	if client, err = fetcher.getClient(ctx); err != nil {
		return nil, "", err
	}

	var bucketName, objectKey = fetcher.getBucketAndObjectKeyFromURL(c, url)
	if bucketName == "" || objectKey == "" {
		return nil, "", fmt.Errorf("Failed to parse file URL '%s'", url)
	}

	c.Logger().Debugf("bucketName=%s  objectKey=%s", bucketName, objectKey)

	var bucket = client.Bucket(bucketName)
	if bucket == nil {
		return nil, "", fmt.Errorf("Failed to obtain access to bucket '%s'", bucketName)
	}
	var obj *storage.ObjectHandle
	var objAttrs *storage.ObjectAttrs
	// The URL contains a leading "/" as part of the path, the API doesn't need it
	obj = bucket.Object(objectKey[1:])

	if objAttrs, err = obj.Attrs(ctx); err != nil {
		c.Logger().Errorf("Failed to fetch object attributes. Reason=%s", err)
		return nil, "", err
	}

	var contentType = objAttrs.ContentType

	var reader *storage.Reader
	if reader, err = obj.NewReader(ctx); err != nil {
		return nil, "", err
	}
	defer reader.Close()

	var buf []byte
	if buf, err = ioutil.ReadAll(reader); err != nil {
		return nil, "", err
	}

	return bytes.NewReader(buf), contentType, nil
}

// NewGoogleStroageFetcher creates a new fetcher that support Google Storage buckets
func NewGoogleStroageFetcher(cfg *config.Config) *GoogleStroageFetcher {
	projectID := cfg.GetFetcherConfigKeyValue("gs", "projectId")
	securitySource := cfg.GetFetcherConfigKeyValue("gs", "securitySource")
	serviceAccountJSONFile := cfg.GetFetcherConfigKeyValue("gs", "serviceAccountJSONFile")

	return &GoogleStroageFetcher{
		ProjectID:              projectID,
		SecuritySource:         securitySource,
		ServiceAccountJSONFile: serviceAccountJSONFile,
	}
}
