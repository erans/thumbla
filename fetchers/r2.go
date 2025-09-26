package fetchers

import (
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

	cfg, err := config.LoadDefaultConfig(c.Context(),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			f.accessKeyID,
			f.secretAccessKey,
			"",
		)),
		config.WithRegion("auto"),
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

	return output.Body, aws.ToString(output.ContentType), nil
}

// GetName returns the name assigned to this fetcher that can be used in the 'paths' section
func (fetcher *CloudflareR2Fetcher) GetName() string {
	return fetcher.Name
}

// GetFetcherType returns the type of this fetcher to be used in the 'type' properties when defining fetchers
func (fetcher *CloudflareR2Fetcher) GetFetcherType() string {
	return fetcher.FetcherType
}
