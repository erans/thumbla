package middleware

import (
	"io"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Logger key for context
const LoggerKey = "logger"

// ZerologConfig defines the config for Zerolog middleware
type ZerologConfig struct {
	// Logger instance to use
	Logger *zerolog.Logger

	// Skip logging for certain paths
	Skip func(*fiber.Ctx) bool

	// GetLogger allows customizing the logger for each request
	GetLogger func(*fiber.Ctx) *zerolog.Logger
}

// ConfigDefault is the default config
var ConfigDefault = ZerologConfig{
	Logger: nil,
	Skip:   nil,
	GetLogger: func(c *fiber.Ctx) *zerolog.Logger {
		return GetLoggerFromContext(c)
	},
}

// New creates a new Zerolog middleware
func New(config ...ZerologConfig) fiber.Handler {
	// Set default config
	cfg := ConfigDefault

	// Override config if provided
	if len(config) > 0 {
		cfg = config[0]
	}

	// Set default logger if not provided
	if cfg.Logger == nil {
		logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
		cfg.Logger = &logger
	}

	return func(c *fiber.Ctx) error {
		// Skip logging if configured
		if cfg.Skip != nil && cfg.Skip(c) {
			return c.Next()
		}

		start := time.Now()

		// Create request logger with context
		reqLogger := cfg.Logger.With().
			Str("method", c.Method()).
			Str("path", c.Path()).
			Str("ip", c.IP()).
			Str("user_agent", c.Get("User-Agent")).
			Str("request_id", c.Get("X-Request-ID")).
			Logger()

		// Store logger in context
		c.Locals(LoggerKey, &reqLogger)

		// Process request
		err := c.Next()

		// Log request completion
		duration := time.Since(start)
		status := c.Response().StatusCode()

		logEvent := reqLogger.Info()
		if status >= 400 {
			logEvent = reqLogger.Warn()
		}
		if status >= 500 {
			logEvent = reqLogger.Error()
		}

		logEvent.
			Int("status", status).
			Dur("duration", duration).
			Int("size", len(c.Response().Body())).
			Msg("Request completed")

		return err
	}
}

// GetLoggerFromContext retrieves the logger from Fiber context
func GetLoggerFromContext(c *fiber.Ctx) *zerolog.Logger {
	if logger, ok := c.Locals(LoggerKey).(*zerolog.Logger); ok {
		return logger
	}
	// Fallback to global logger
	return &log.Logger
}

// InitGlobalLogger initializes the global zerolog logger
func InitGlobalLogger(level string, pretty bool) {
	// Parse log level
	logLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(logLevel)

	// Configure output
	var output io.Writer = os.Stdout
	if pretty {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
	}

	// Set global logger
	log.Logger = zerolog.New(output).With().
		Timestamp().
		Caller().
		Logger()
}