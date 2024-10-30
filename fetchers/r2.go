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

// CloudflareR2Fetcher implements Fetcher interface for Cloudflare R2
type CloudflareR2Fetcher struct {
	accessKeyID     string
	secretAccessKey string
	accountID       string
	bucket          string
}

// NewCloudflareR2Fetcher creates a new Cloudflare R2 fetcher instance
func NewCloudflareR2Fetcher(accessKeyID, secretAccessKey, accountID, bucket string) *CloudflareR2Fetcher {
	return &CloudflareR2Fetcher{
		accessKeyID:     accessKeyID,
		secretAccessKey: secretAccessKey,
		accountID:       accountID,
		bucket:          bucket,
	}
}

// Fetch downloads an object from Cloudflare R2
func (f *CloudflareR2Fetcher) Fetch(ctx context.Context, path string) (io.ReadCloser, error) {
	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: fmt.Sprintf("https://%s.r2.cloudflarestorage.com", f.accountID),
		}, nil
	})

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("auto"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(f.accessKeyID, f.secretAccessKey, "")),
		config.WithEndpointResolverWithOptions(resolver),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load R2 config: %v", err)
	}

	client := s3.NewFromConfig(cfg)

	output, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(f.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object from R2: %v", err)
	}

	return output.Body, nil
}
