package fetchers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"path"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"

	"github.com/erans/thumbla/utils"
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
	Name        string
	FetcherType string
	Bucket      string
	Path        string

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

	if options != nil {
		return storage.NewClient(ctx, options)
	}

	return storage.NewClient(ctx)
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
		c.Logger().Debugf("Failed to get bucket and object key from URL, assume this is a relative one")
		bucketName = fetcher.Bucket
		objectKey = path.Join(fetcher.Path, url)

		c.Logger().Debugf("bucketName=%s objectKey=%s", bucketName, objectKey)

		if bucketName == "" || objectKey == "" {
			return nil, "", fmt.Errorf("Failed to parse file URL '%s'", url)
		}
	}

	c.Logger().Debugf("bucketName=%s  objectKey=%s", bucketName, objectKey)

	var bucket = client.Bucket(bucketName)
	if bucket == nil {
		return nil, "", fmt.Errorf("Failed to obtain access to bucket '%s'", bucketName)
	}
	var obj *storage.ObjectHandle
	var objAttrs *storage.ObjectAttrs
	var objectPath = objectKey
	if objectKey[0] == '/' {
		// The URL contains a leading "/" as part of the path, the API doesn't need it
		objectPath = objectKey[1:]
	}

	c.Logger().Debugf("objectPath=%s", objectPath)

	obj = bucket.Object(objectPath)

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

// GetName returns the name assigned to this fetcher that can be used in the 'paths' section
func (fetcher *GoogleStroageFetcher) GetName() string {
	return fetcher.Name
}

// GetFetcherType returns the type of this fetcher to be used in the 'type' properties when defining fetchers
func (fetcher *GoogleStroageFetcher) GetFetcherType() string {
	return fetcher.FetcherType
}

// NewGoogleStroageFetcher creates a new fetcher that support Google Storage buckets
func NewGoogleStroageFetcher(cfg map[string]interface{}) *GoogleStroageFetcher {
	var name, _ = cfg["name"]
	var projectID, _ = cfg["projectId"]
	var securitySource, _ = cfg["securitySource"]
	var serviceAccountJSONFile, _ = cfg["serviceAccountJSONFile"]
	var bucket, _ = cfg["bucket"]
	var path, _ = cfg["path"]

	return &GoogleStroageFetcher{
		Name:                   utils.SafeCastToString(name),
		FetcherType:            "gs",
		Bucket:                 utils.SafeCastToString(bucket),
		Path:                   utils.SafeCastToString(path),
		ProjectID:              utils.SafeCastToString(projectID),
		SecuritySource:         utils.SafeCastToString(securitySource),
		ServiceAccountJSONFile: utils.SafeCastToString(serviceAccountJSONFile),
	}
}
