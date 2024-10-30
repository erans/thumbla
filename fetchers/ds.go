package fetchers

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/erans/thumbla/utils"
	"github.com/labstack/echo/v4"
)

// DigitalOceanSpacesFetcher implements Fetcher interface for DigitalOcean Spaces
type DigitalOceanSpacesFetcher struct {
	Name      string
	accessKey string
	secretKey string
	region    string
	bucket    string
	endpoint  string
}

// NewDigitalOceanSpacesFetcher creates a new DigitalOcean Spaces fetcher instance
func NewDigitalOceanSpacesFetcher(cfg map[string]interface{}) *DigitalOceanSpacesFetcher {
	var name, _ = cfg["name"]
	var accessKey, _ = cfg["accessKey"]
	var secretKey, _ = cfg["secretKey"]
	var region, _ = cfg["region"]
	var bucket, _ = cfg["bucket"]
	var endpoint, _ = cfg["endpoint"]

	return &DigitalOceanSpacesFetcher{
		Name:      utils.SafeCastToString(name),
		accessKey: utils.SafeCastToString(accessKey),
		secretKey: utils.SafeCastToString(secretKey),
		region:    utils.SafeCastToString(region),
		bucket:    utils.SafeCastToString(bucket),
		endpoint:  utils.SafeCastToString(endpoint),
	}
}

// Fetch downloads an object from DigitalOcean Spaces
func (f *DigitalOceanSpacesFetcher) Fetch(c echo.Context, path string) (io.Reader, string, error) {
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: f.endpoint,
		}, nil
	})

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			f.accessKey,
			f.secretKey,
			"",
		)),
		config.WithRegion(f.region),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load config: %v", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	output, err := client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(f.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get object from Spaces: %v", err)
	}

	return output.Body, aws.ToString(output.ContentType), nil
}

// GetName returns the name assigned to this fetcher
func (f *DigitalOceanSpacesFetcher) GetName() string {
	return f.Name
}

// GetFetcherType returns the type of this fetcher
func (f *DigitalOceanSpacesFetcher) GetFetcherType() string {
	return "spaces"
}
