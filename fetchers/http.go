package fetchers

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/erans/thumbla/config"
	"github.com/erans/thumbla/utils"
	"github.com/klauspost/compress/zstd"
	"github.com/gofiber/fiber/v2"
)

// HTTPFetcher fetches content from http/https sources
type HTTPFetcher struct {
	Name        string
	FetcherType string

	UserName string
	Password string

	Secure                 bool
	RestrictHosts          []string
	RestrictPaths          []string
	DisableSSRFProtection  bool
}

// isPrivateOrLocalIP checks if an IP address is private, localhost, or link-local
func isPrivateOrLocalIP(ip net.IP) bool {
	// Define private IP ranges
	privateRanges := []net.IPNet{
		{IP: net.IPv4(10, 0, 0, 0), Mask: net.IPv4Mask(255, 0, 0, 0)},         // 10.0.0.0/8
		{IP: net.IPv4(172, 16, 0, 0), Mask: net.IPv4Mask(255, 240, 0, 0)},     // 172.16.0.0/12
		{IP: net.IPv4(192, 168, 0, 0), Mask: net.IPv4Mask(255, 255, 0, 0)},    // 192.168.0.0/16
		{IP: net.IPv4(127, 0, 0, 0), Mask: net.IPv4Mask(255, 0, 0, 0)},        // 127.0.0.0/8 (localhost)
		{IP: net.IPv4(169, 254, 0, 0), Mask: net.IPv4Mask(255, 255, 0, 0)},    // 169.254.0.0/16 (link-local)
	}

	// Check IPv4 private ranges
	for _, privateRange := range privateRanges {
		if privateRange.Contains(ip) {
			return true
		}
	}

	// Check IPv6 localhost and private ranges
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	return false
}

// isDangerousPort checks if a port is commonly used for internal services
func isDangerousPort(port string) bool {
	dangerousPorts := map[string]bool{
		"22":   true, // SSH
		"23":   true, // Telnet
		"25":   true, // SMTP
		"53":   true, // DNS
		"110":  true, // POP3
		"143":  true, // IMAP
		"993":  true, // IMAPS
		"995":  true, // POP3S
		"1433": true, // MSSQL
		"3306": true, // MySQL
		"5432": true, // PostgreSQL
		"6379": true, // Redis
		"11211": true, // Memcached
		"27017": true, // MongoDB
	}
	return dangerousPorts[port]
}

// Fetch returns content from http/https sources
func (fetcher *HTTPFetcher) Fetch(c *fiber.Ctx, fetchURL string) (io.Reader, string, error) {
	var request *http.Request
	var response *http.Response
	var err error

	// Parse URL for validation
	parsedURL, err := url.Parse(fetchURL)
	if err != nil {
		return nil, "", fmt.Errorf("invalid URL: %w", err)
	}

	// SSRF Protection: Resolve hostname to IP and check for private/local addresses
	// Skip SSRF protection if disabled (for testing)
	if !fetcher.DisableSSRFProtection {
		host := parsedURL.Hostname()
		port := parsedURL.Port()

		// Check for dangerous ports
		if port != "" && isDangerousPort(port) {
			return nil, "", fmt.Errorf("access to port %s is not allowed for security reasons", port)
		}

		// Resolve hostname to IP addresses
		ips, err := net.LookupIP(host)
		if err != nil {
			return nil, "", fmt.Errorf("failed to resolve hostname %s: %w", host, err)
		}

		// Check if any resolved IP is private or local
		for _, ip := range ips {
			if isPrivateOrLocalIP(ip) {
				return nil, "", fmt.Errorf("access to private/local IP %s (resolved from %s) is not allowed for security reasons", ip.String(), host)
			}
		}
	}

	// Check host restrictions
	if len(fetcher.RestrictHosts) > 0 {
		hostAllowed := false
		for _, allowedHost := range fetcher.RestrictHosts {
			if parsedURL.Host == allowedHost || strings.HasSuffix(parsedURL.Host, "."+allowedHost) {
				hostAllowed = true
				break
			}
		}
		if !hostAllowed {
			return nil, "", fmt.Errorf("host %s is not in allowed hosts list", parsedURL.Host)
		}
	}

	// Check path restrictions
	if len(fetcher.RestrictPaths) > 0 {
		pathAllowed := false
		for _, allowedPath := range fetcher.RestrictPaths {
			if strings.HasPrefix(parsedURL.Path, allowedPath) {
				pathAllowed = true
				break
			}
		}
		if !pathAllowed {
			return nil, "", fmt.Errorf("path %s is not in allowed paths list", parsedURL.Path)
		}
	}

	client := &http.Client{
		Timeout: time.Duration(config.GetConfig().GetHTTPTimeout()) * time.Second,
	}

	// Create context with timeout for better resource management
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(config.GetConfig().GetHTTPTimeout()) * time.Second)
	defer cancel() // Ensure context is cancelled to prevent resource leaks

	if request, err = http.NewRequestWithContext(ctx, "GET", fetchURL, nil); err != nil {
		return nil, "", err
	}
	request.Header.Add("Accept-Encoding", "gzip, compress, br, zstd")

	// Add basic auth if username and password are configured
	if fetcher.UserName != "" || fetcher.Password != "" {
		request.SetBasicAuth(fetcher.UserName, fetcher.Password)
	}

	if response, err = client.Do(request); err != nil {
		return nil, "", err
	}

	defer response.Body.Close()

	// Check HTTP status code
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, "", fmt.Errorf("HTTP error: %d %s", response.StatusCode, response.Status)
	}

	var reader io.Reader
	switch response.Header.Get("Content-Encoding") {
	case "gzip":
		gzReader, err := gzip.NewReader(response.Body)
		if err != nil {
			return nil, "", err
		}
		defer gzReader.Close()
		reader = gzReader
	case "br":
		reader = brotli.NewReader(response.Body)
		// No need for Close() for brotli reader
	case "zstd":
		zstdReader, err := zstd.NewReader(response.Body)
		if err != nil {
			return nil, "", err
		}
		defer zstdReader.Close()
		reader = zstdReader
	case "deflate":
		reader = flate.NewReader(response.Body)
		defer reader.(io.ReadCloser).Close()
	default:
		reader = response.Body
	}

	// Check content length before reading to prevent memory exhaustion
	if response.ContentLength > 0 {
		maxSize := config.GetConfig().GetMaxImageSizeBytes()
		if response.ContentLength > maxSize {
			return nil, "", fmt.Errorf("image size (%d bytes) exceeds maximum allowed size (%d bytes)",
				response.ContentLength, maxSize)
		}
	}

	var buf []byte
	if buf, err = io.ReadAll(io.LimitReader(reader, config.GetConfig().GetMaxImageSizeBytes())); err != nil {
		return nil, "", err
	}

	// Double-check actual size read
	if int64(len(buf)) > config.GetConfig().GetMaxImageSizeBytes() {
		return nil, "", fmt.Errorf("image size (%d bytes) exceeds maximum allowed size (%d bytes)",
			len(buf), config.GetConfig().GetMaxImageSizeBytes())
	}

	var contentType = response.Header.Get("Content-Type")
	log.Printf("Fetched %s Content-Type=%s", fetchURL, contentType)

	return bytes.NewReader(buf), contentType, nil
}

// GetName returns the name assigned to this fetcher that can be used in the 'paths' section
func (fetcher *HTTPFetcher) GetName() string {
	return fetcher.Name
}

// GetFetcherType returns the type of this fetcher to be used in the 'type' properties when defining fetchers
func (fetcher *HTTPFetcher) GetFetcherType() string {
	return fetcher.FetcherType
}

// NewHTTPFetcher creates a new fetcher that support http/https
func NewHTTPFetcher(cfg map[string]interface{}) *HTTPFetcher {
	var ok bool

	var name, _ = cfg["name"]
	var username, _ = cfg["username"]
	var password, _ = cfg["password"]
	var secure bool
	if secure, ok = cfg["secure"].(bool); !ok {
		secure = false
	}
	var restrictHosts []string
	if hostsInterface, ok := cfg["restrictHosts"]; ok {
		if hostsList, ok := hostsInterface.([]interface{}); ok {
			for _, host := range hostsList {
				if hostStr, ok := host.(string); ok {
					restrictHosts = append(restrictHosts, hostStr)
				}
			}
		} else if hostsList, ok := hostsInterface.([]string); ok {
			restrictHosts = hostsList
		}
	}

	var restrictPaths []string
	if pathsInterface, ok := cfg["restrictPaths"]; ok {
		if pathsList, ok := pathsInterface.([]interface{}); ok {
			for _, path := range pathsList {
				if pathStr, ok := path.(string); ok {
					restrictPaths = append(restrictPaths, pathStr)
				}
			}
		} else if pathsList, ok := pathsInterface.([]string); ok {
			restrictPaths = pathsList
		}
	}

	var disableSSRFProtection bool
	if disableSSRFInterface, ok := cfg["disableSSRFProtection"]; ok {
		if disableSSRF, ok := disableSSRFInterface.(bool); ok {
			disableSSRFProtection = disableSSRF
		}
	}

	return &HTTPFetcher{
		Name:                  utils.SafeCastToString(name),
		FetcherType:           "http",
		UserName:              utils.SafeCastToString(username),
		Password:              utils.SafeCastToString(password),
		Secure:                secure,
		RestrictHosts:         restrictHosts,
		RestrictPaths:         restrictPaths,
		DisableSSRFProtection: disableSSRFProtection,
	}
}
