package utils

import "strings"

// GetMimeTypeByFileExt returns a file's mime type based on the file extension (.jpg,.png, etc)
func GetMimeTypeByFileExt(url string) string {
	parts := strings.Split(url, ".")
	var fileExt = strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))

	if fileExt == "jpg" || fileExt == "jpeg" {
		return "image/jpg"
	} else if fileExt == "png" {
		return "image/png"
	} else if fileExt == "webp" {
		return "image/webp"
	} else if fileExt == "svg" {
		return "image/svg+xml"
	}

	return ""
}
