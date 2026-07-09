package fetchers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"regexp"
	"strings"

	"github.com/erans/thumbla/config"
	"github.com/erans/thumbla/utils"
	"github.com/gofiber/fiber/v2"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Fetcher fetches content from an s3 bucket
type S3Fetcher struct {
	Name            string
	FetcherType     string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
}

var awsRegionRegExp, _ = regexp.Compile(`s3-(.*)\.amazonaws\.com`)

func (fetcher *S3Fetcher) getBucketAndObjectKeyFromURL(c *fiber.Ctx, fileURL string) (string, string, string) {
	if u, err := url.Parse(fileURL); err == nil {
		// Parse the following format:
		// - http://s3-aws-region.amazonaws.com/bucket/path/file  i.e. http://s3-us-west-2.amazonaws.com/mybucket/path/file

		var region string
		var match = awsRegionRegExp.FindStringSubmatch(u.Host)
		if len(match) > 1 {
			region = match[1]
			log.Printf("Found region in URL '%s'", region)
		}

		if region == "" {
			// If we couldn't get a region, assume the default configured region should be used
			region = fetcher.Region
			log.Printf("No region found, using default '%s'", fetcher.Region)
		}

		var bucket string
		var objectKey string

		parts := strings.Split(u.Path, "/")
		log.Printf("URL parts: %v", parts)

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
func (fetcher *S3Fetcher) Fetch(c *fiber.Ctx, fileURL string) (io.Reader, string, error) {
	log.Printf("Fetching from S3: %s", fileURL)

	region, bucket, objectKey := fetcher.getBucketAndObjectKeyFromURL(c, fileURL)
	log.Printf("Region: %s   Bucket: %s  ObjectKey: %s", region, bucket, objectKey)

	if region == "" || bucket == "" || objectKey == "" {
		return nil, "", fmt.Errorf("failed to parse file URL. url=%s", fileURL)
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			fetcher.AccessKeyID,
			fetcher.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load AWS config: %v", err)
	}

	client := s3.NewFromConfig(cfg)

	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	}

	output, err := client.GetObject(context.TODO(), input)
	if err != nil {
		return nil, "", err
	}

	defer output.Body.Close()

	maxSize := config.GetConfig().GetMaxImageSizeBytes()
	contentLength := int64(0)
	if output.ContentLength != nil {
		contentLength = *output.ContentLength
		if contentLength > maxSize {
			return nil, "", fmt.Errorf("object size (%d bytes) exceeds maximum (%d bytes)", contentLength, maxSize)
		}
	}

	log.Printf("Content Length: %d    Content-Type: %s", contentLength, aws.ToString(output.ContentType))

	buf := new(bytes.Buffer)
	n, err := io.Copy(buf, io.LimitReader(output.Body, maxSize+1))
	if err != nil {
		return nil, "", err
	}
	if n > maxSize {
		return nil, "", fmt.Errorf("object size exceeds maximum (%d bytes)", maxSize)
	}

	return bytes.NewReader(buf.Bytes()), aws.ToString(output.ContentType), nil
}

// GetName returns the name assigned to this fetcher that can be used in the 'paths' section
func (fetcher *S3Fetcher) GetName() string {
	return fetcher.Name
}

// GetFetcherType returns the type of this fetcher to be used in the 'type' properties when defining fetchers
func (fetcher *S3Fetcher) GetFetcherType() string {
	return fetcher.FetcherType
}

// NewS3Fetcher creates a new fetcher that support s3 bucket
func NewS3Fetcher(cfg map[string]interface{}) *S3Fetcher {
	var name, _ = cfg["name"]
	var region, _ = cfg["region"]
	var accessKeyID, _ = cfg["accessKeyId"]
	var secretAccessKey, _ = cfg["secretAccessKey"]

	return &S3Fetcher{
		Name:            utils.SafeCastToString(name),
		FetcherType:     "s3",
		Region:          utils.SafeCastToString(region),
		AccessKeyID:     utils.SafeCastToString(accessKeyID),
		SecretAccessKey: utils.SafeCastToString(secretAccessKey),
	}
}
