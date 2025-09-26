# Claude Code Configuration for Thumbla

This file contains project-specific information and commands for Claude Code to efficiently work with the Thumbla image processing microservice.

## Project Overview

Thumbla is a versatile image processing microservice written in Go that provides secure access to images stored in private cloud storage locations with extensive transformation capabilities.

**Key Features:**
- Multi-format image support (JPEG, PNG, WEBP, GIF, SVG)
- Multiple storage backends (AWS S3, Google Storage, Azure Blob, DigitalOcean Spaces, Cloudflare R2, Local, HTTP/S)
- Real-time image manipulations (resize, crop, rotate, face detection, etc.)
- Currently uses Echo v4 web framework

## Tech Stack

- **Language:** Go 1.25.1
- **Web Framework:** Echo v4 (labstack/echo/v4)
- **CLI:** Kingpin v2
- **Image Processing:** Multiple libraries including bild, oksvg, go-webp
- **Cloud Storage:** AWS SDK v2, Google Cloud Storage, Azure Blob Storage
- **Caching:** Redis support via go-redis/v9

## Project Structure

- `thumbla.go` - Main application entry point
- `config/` - Configuration management
- `handlers/` - HTTP request handlers
- `fetchers/` - Image source adapters (S3, GCS, Azure, etc.)
- `manipulators/` - Image transformation logic
- `cache/` - Caching implementation

## Development Commands

### Build & Run
```bash
# Build the project
go build

# Run with config file
./thumbla -c config-example.yml

# Run with custom host/port
./thumbla -c config.yml -o 0.0.0.0 -p 8080
```

### Dependencies
```bash
# Update dependencies
go mod tidy

# Add new dependency
go get <package>

# Check for vulnerabilities
go mod download && go list -json -m all | nancy sleuth
```

### Testing
```bash
# Currently no tests available
# Consider adding tests when implementing new features
```

### Configuration
- Main config file: `config-example.yml` (copy to create your own)
- Environment variables: `THUMBLACFG`, `HOST`, `PORT`
- The service requires a configuration file to specify fetchers and paths

## Common Tasks

### Adding New Image Manipulator
1. Create new manipulator in `manipulators/` directory
2. Implement the required interface
3. Register in the manipulator factory
4. Update URL parsing logic in handlers

### Adding New Storage Backend
1. Create new fetcher in `fetchers/` directory
2. Implement the Fetcher interface
3. Add configuration options to config package
4. Register in fetcher factory

### URL Structure
```
https://domain.com/{configured-path}/{encoded-source-path}/{manipulations}/output:f={format}

Example:
https://example.com/i/pics/path%2Fto%2Fimage.jpg/resize:w=350/output:f=jpg
```

## Current Migration Plan

**Goal:** Migrate from Echo v4 to Fiber v3 web framework

**Key Areas to Update:**
- Replace Echo imports with Fiber
- Update middleware configuration
- Modify handler signatures and response methods
- Update route definitions
- Test compatibility with existing image processing pipeline

## Notes

- The project follows a modular architecture with clear separation between fetchers, manipulators, and handlers
- All image paths in URLs must be URL-encoded
- The service supports both public and private cloud storage access
- Face detection capabilities require additional cloud service configuration (AWS Rekognition, Google Vision, Azure Face API)