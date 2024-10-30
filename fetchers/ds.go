package fetchers

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// DigitalOceanSpacesFetcher implements Fetcher interface for DigitalOcean Spaces
type DigitalOceanSpacesFetcher struct {
	accessKey string
	secretKey string
	region    string
	bucket    string
	endpoint  string
}

// NewDigitalOceanSpacesFetcher creates a new DigitalOcean Spaces fetcher instance
func NewDigitalOceanSpacesFetcher(accessKey, secretKey, region, bucket, endpoint string) *DigitalOceanSpacesFetcher {
	return &DigitalOceanSpacesFetcher{
		accessKey: accessKey,
		secretKey: secretKey,
		region:    region,
		bucket:    bucket,
		endpoint:  endpoint,
	}
}

// Fetch downloads an object from DigitalOcean Spaces
func (f *DigitalOceanSpacesFetcher) Fetch(ctx context.Context, path string) (io.ReadCloser, error) {
	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: f.endpoint,
		}, nil
	})

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(f.region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(f.accessKey, f.secretKey, "")),
		config.WithEndpointResolverWithOptions(resolver),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %v", err)
	}

	client := s3.NewFromConfig(cfg)

	output, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(f.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object from Spaces: %v", err)
	}

	return output.Body, nil
}
