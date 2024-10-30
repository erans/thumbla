package fetchers

import (
	"context"
	"fmt"
	"io"
	"net/url"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

// AzureStorageFetcher implements Fetcher interface for Azure Storage blobs
type AzureStorageFetcher struct {
	accountName   string
	accountKey    string
	containerName string
}

// NewAzureStorageFetcher creates a new Azure Storage fetcher instance
func NewAzureStorageFetcher(accountName, accountKey, containerName string) *AzureStorageFetcher {
	return &AzureStorageFetcher{
		accountName:   accountName,
		accountKey:    accountKey,
		containerName: containerName,
	}
}

// Fetch downloads a blob from Azure Storage
func (f *AzureStorageFetcher) Fetch(ctx context.Context, path string) (io.ReadCloser, error) {
	credential, err := azblob.NewSharedKeyCredential(f.accountName, f.accountKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create credential: %v", err)
	}

	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})

	URL, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s", f.accountName, f.containerName))
	containerURL := azblob.NewContainerURL(*URL, p)
	blobURL := containerURL.NewBlobURL(path)

	response, err := blobURL.Download(ctx, 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false, azblob.ClientProvidedKeyOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to download blob: %v", err)
	}

	return response.Body(azblob.RetryReaderOptions{}), nil
}
