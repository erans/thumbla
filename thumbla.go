package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/erans/thumbla/cache"
	"github.com/erans/thumbla/config"
	"github.com/erans/thumbla/fetchers"
	"github.com/erans/thumbla/handlers"
	"github.com/erans/thumbla/manipulators"
	"github.com/erans/thumbla/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/rs/zerolog/log"

	kingpin "github.com/alecthomas/kingpin/v2"
)

var (
	configFile = kingpin.Flag("config", "Configuration file").Short('c').OverrideDefaultFromEnvar("THUMBLACFG").Required().String()
	host       = kingpin.Flag("host", "Host to listen on").Short('o').OverrideDefaultFromEnvar("HOST").Default("127.0.0.1").String()
	port       = kingpin.Flag("port", "Listening Port").Short('p').OverrideDefaultFromEnvar("PORT").Default("1323").String()
)

// Initialize zerolog based on debug level
func initLogging(cfg *config.Config) {
	middleware.InitGlobalLogger(cfg.DebugLevel, true)
}

func main() {
	kingpin.Parse()

	var cfg *config.Config
	var err error
	if cfg, err = config.LoadConfig(*configFile); err != nil {
		fmt.Printf("Failed to load config. Exiting. Reason %v", err)
		return
	}

	// Set currently active global config
	config.SetConfig(cfg)

	// Initialize logging
	initLogging(cfg)

	// Init all registered fetchers
	fetchers.InitFetchers(cfg)

	// Init all registered manipulators
	manipulators.InitManipulators(cfg)

	// Init the cache handlers
	cache.InitCache(cfg)

	app := fiber.New(fiber.Config{
		BodyLimit:    int(cfg.GetMaxRequestSize()),
		ReadTimeout:  time.Duration(cfg.GetReadTimeout()) * time.Second,
		WriteTimeout: time.Duration(cfg.GetWriteTimeout()) * time.Second,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			logger := middleware.GetLoggerFromContext(c)
			logger.Error().Err(err).Msg("Request error")
			return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
		},
	})

	// Use zerolog middleware instead of default logger
	app.Use(middleware.New())
	app.Use(recover.New())

	// Add rate limiting if enabled
	if cfg.IsRateLimitEnabled() {
		app.Use(limiter.New(limiter.Config{
			Max:        cfg.GetRateLimitMaxRequests(),
			Expiration: time.Duration(cfg.GetRateLimitWindow()) * time.Second,
			KeyGenerator: func(c *fiber.Ctx) string {
				return c.IP() // Rate limit per IP address
			},
			LimitReached: func(c *fiber.Ctx) error {
				logger := middleware.GetLoggerFromContext(c)
				logger.Warn().
					Str("ip", c.IP()).
					Int("limit", cfg.GetRateLimitMaxRequests()).
					Int("window", cfg.GetRateLimitWindow()).
					Msg("Rate limit exceeded")
				return c.Status(fiber.StatusTooManyRequests).SendString("Rate limit exceeded")
			},
		}))
		log.Info().
			Int("maxRequests", cfg.GetRateLimitMaxRequests()).
			Int("windowSec", cfg.GetRateLimitWindow()).
			Msg("Rate limiting enabled")
	}

	app.Get("/health", handlers.HandleHealth)

	for _, p := range cfg.Paths {
		var path = p.Path
		if strings.Index(path, ":url") == -1 {
			path = fmt.Sprintf("%s/:url/*", path)
		}
		app.Get(path, handlers.HandleImage)
	}

	log.Info().Str("host", *host).Str("port", *port).Msg("Starting Thumbla server")
	if err := app.Listen(fmt.Sprintf("%s:%s", *host, *port)); err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}
