package main

import (
	"fmt"
	"strings"

	"github.com/erans/thumbla/cache"
	"github.com/erans/thumbla/config"
	"github.com/erans/thumbla/fetchers"
	"github.com/erans/thumbla/handlers"
	"github.com/erans/thumbla/manipulators"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/labstack/gommon/log"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	configFile = kingpin.Flag("config", "Configuration file").Short('c').OverrideDefaultFromEnvar("THUMBLACFG").Required().String()
	host       = kingpin.Flag("host", "Host to listen on").Short('o').OverrideDefaultFromEnvar("HOST").Default("127.0.0.1").String()
	port       = kingpin.Flag("port", "Listening Port").Short('p').OverrideDefaultFromEnvar("PORT").Default("1323").String()
)

var debugLevels = map[string]log.Lvl{
	"debug": log.DEBUG,
	"info":  log.INFO,
	"warn":  log.WARN,
	"error": log.ERROR,
	"off":   log.OFF,
}

func getDebugLevelByName(name string) log.Lvl {
	if val, ok := debugLevels[name]; ok {
		return val
	}

	return log.ERROR
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

	// Init all registered fetchers
	fetchers.InitFetchers(cfg)

	// Init all registered manipulators
	manipulators.InitManipulators(cfg)

	// Init the cache handlers
	cache.InitCache(cfg)

	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.Logger.SetLevel(getDebugLevelByName(cfg.DebugLevel))

	e.GET("/health", handlers.HandleHealth)

	for _, p := range cfg.Paths {
		var path = p.Path
		if strings.Index(path, ":url") == -1 {
			path = fmt.Sprintf("%s/:url/*", path)
		}
		e.GET(path, handlers.HandleImage)
	}

	e.Logger.Fatal(e.Start(fmt.Sprintf("%s:%s", *host, *port)))
}
