package fetchers

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"github.com/erans/thumbla/config"
	"github.com/labstack/echo"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// S3Fetcher fetches content from an s3 bucket
type S3Fetcher struct {
	Region string
}

var awsRegionRegExp, _ = regexp.Compile("s3-(.*)\\.amazonaws\\.com")

func (fetcher *S3Fetcher) getBucketAndObjectKeyFromURL(c echo.Context, fileURL string) (string, string, string) {
	if u, err := url.Parse(fileURL); err == nil {
		// Parse the following format:
		// - http://s3-aws-region.amazonaws.com/bucket/path/file  i.e. http://s3-us-west-2.amazonaws.com/mybucket/path/file

		var region string
		var match = awsRegionRegExp.FindStringSubmatch(u.Host)
		if len(match) > 1 {
			region = match[1]
			c.Logger().Debugf("Found region in URL '%s'", region)
		}

		if region == "" {
			// If we couldn't get a region, assume the default configured region should be used
			region = fetcher.Region
			c.Logger().Debugf("No region found, using default '%s'", fetcher.Region)
		}

		var bucket string
		var objectKey string

		parts := strings.Split(u.Path, "/")
		c.Logger().Debugf("URL parts: %v", parts)

		// The minimum would be /bucket/file which would translate into an array
		// with 0 == empty string, 1 == bucket and 2 == file
		if len(parts) < 3 {
			return "", "", ""
		}

		bucket = parts[1]
		objectKey = strings.Join(parts[2:], "/")

		return region, bucket, objectKey
	}

	return "", "", ""
}

// Fetch returns content from the local machine
//
// It supports the following S3 URL format:
// - path style: s3://s3-aws-region.amazonaws.com/bucket/path/file
// - path style without region: s3://s3.amazonaws.com/bucket/path/file - default region must be configured in the configuration
// The bucket and path will be extracted from the URL.
//
// If you are accessing an S3 file that is accessible via anonymous direct HTTP/S
// consider using the http fetcher.
func (fetcher *S3Fetcher) Fetch(c echo.Context, fileURL string) (io.Reader, string, error) {
	c.Logger().Debugf("Fetching from S3: %s", fileURL)

	region, bucket, objectKey := fetcher.getBucketAndObjectKeyFromURL(c, fileURL)
	c.Logger().Debugf("Region: %s   Bucket: %s  ObjectKey: %s", region, bucket, objectKey)

	var sess *session.Session
	var err error
	sess, err = session.NewSession()
	if err != nil {
		return nil, "", fmt.Errorf("Failed to create S3 session")
	}

	if region == "" || bucket == "" || objectKey == "" {
		return nil, "", fmt.Errorf("Failed to parse file URL. url=%s", fileURL)
	}

	svc := s3.New(sess, &aws.Config{Region: aws.String(region)})

	params := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	}

	// Get object info including its size
	var s3obj *s3.GetObjectOutput
	if s3obj, err = svc.GetObject(params); err != nil {
		return nil, "", nil
	}

	// TODO: Currently we fetch the image to the memory. Consider adding protection to limit the max size

	c.Logger().Debugf("Content Length: %d    Content-Type: %s", *s3obj.ContentLength, *s3obj.ContentType)

	buf := make([]byte, *s3obj.ContentLength)
	downloader := s3manager.NewDownloaderWithClient(svc)
	if _, err = downloader.Download(aws.NewWriteAtBuffer(buf), params); err == nil {
		return bytes.NewReader(buf), *s3obj.ContentType, nil
	}

	return nil, "", err
}

// NewS3Fetcher creates a new fetcher that support s3 bucket
func NewS3Fetcher(cfg *config.Config) *S3Fetcher {
	var region = cfg.GetFetcherConfigKeyValue("s3", "region")
	return &S3Fetcher{Region: region}
}
