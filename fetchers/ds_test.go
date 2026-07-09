package fetchers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/erans/thumbla/config"
)

func TestDigitalOceanSpacesFetcherRejectsOversizedContentLength(t *testing.T) {
	config.SetConfig(&config.Config{
		Server: config.ServerConfig{MaxImageSizeBytes: 5},
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/test-bucket/test.jpg" {
			t.Errorf("expected path /test-bucket/test.jpg, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Length", "6")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("123456"))
	}))
	defer server.Close()

	fetcher := NewDigitalOceanSpacesFetcher(map[string]interface{}{
		"name":      "spaces",
		"type":      "ds",
		"accessKey": "test-access-key",
		"secretKey": "test-secret-key",
		"region":    "us-east-1",
		"bucket":    "test-bucket",
		"endpoint":  server.URL,
	})

	body, contentType, err := fetcher.Fetch(nil, "test.jpg")
	if err == nil {
		t.Fatal("expected oversized object error, got nil")
	}
	if body != nil {
		t.Fatalf("expected nil body on oversized object, got %T", body)
	}
	if contentType != "" {
		t.Fatalf("expected empty content type on oversized object, got %q", contentType)
	}
	if !strings.Contains(err.Error(), "object size (6 bytes) exceeds maximum (5 bytes)") {
		t.Fatalf("expected max-size error, got: %v", err)
	}
}
