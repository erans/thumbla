package fetchers

import (
	"bytes"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/erans/thumbla/config"
	"github.com/erans/thumbla/utils"
	"github.com/gofiber/fiber/v2"
)

// CloudflareR2Fetcher implements Fetcher interface for Cloudflare R2
type CloudflareR2Fetcher struct {
	Name        string
	FetcherType string

	accessKeyID     string
	secretAccessKey string
	accountID       string
	bucket          string
}

// NewCloudflareR2Fetcher creates a new Cloudflare R2 fetcher instance
func NewCloudflareR2Fetcher(cfg map[string]interface{}) *CloudflareR2Fetcher {
	var name, _ = cfg["name"]
	var bucket, _ = cfg["bucket"]
	var accountID, _ = cfg["accountId"]
	var accessKeyID, _ = cfg["accessKeyId"]
	var secretAccessKey, _ = cfg["secretAccessKey"]

	return &CloudflareR2Fetcher{
		Name:            utils.SafeCastToString(name),
		FetcherType:     "r2",
		accessKeyID:     utils.SafeCastToString(accessKeyID),
		secretAccessKey: utils.SafeCastToString(secretAccessKey),
		accountID:       utils.SafeCastToString(accountID),
		bucket:          utils.SafeCastToString(bucket),
	}
}

// Fetch downloads an object from Cloudflare R2
func (f *CloudflareR2Fetcher) Fetch(c *fiber.Ctx, fileURL string) (io.Reader, string, error) {
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: fmt.Sprintf("https://%s.r2.cloudflarestorage.com", f.accountID),
		}, nil
	})

	cfg, err := awsconfig.LoadDefaultConfig(c.Context(),
		awsconfig.WithEndpointResolverWithOptions(customResolver),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			f.accessKeyID,
			f.secretAccessKey,
			"",
		)),
		awsconfig.WithRegion("auto"),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load config: %v", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	output, err := client.GetObject(c.Context(), &s3.GetObjectInput{
		Bucket: aws.String(f.bucket),
		Key:    aws.String(fileURL),
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get object from R2: %v", err)
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
func (fetcher *CloudflareR2Fetcher) GetName() string {
	return fetcher.Name
}

// GetFetcherType returns the type of this fetcher to be used in the 'type' properties when defining fetchers
func (fetcher *CloudflareR2Fetcher) GetFetcherType() string {
	return fetcher.FetcherType
}
