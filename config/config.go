package config

import (
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

var (
	globalConfig *Config
)

// PathConfig represents configuration for a path serving images
type PathConfig struct {
	Path         string `yaml:"path"`
	FetcherName  string `yaml:"fetcherName"`
	CacheControl string `yaml:"cacheControl"`
}

// Config provides a configuration struct for the server
type Config struct {
	CacheControlHeader string                   `yaml:"cacheControlHeader"`
	DebugLevel         string                   `yaml:"debugLevel"`
	Fetchers           []map[string]interface{} `yaml:"fetchers"`
	Paths              []PathConfig             `yaml:"paths"`
	FaceAPI            struct {
		DefaultProvider  string `yaml:"defaultProvider"`
		MicrosoftFaceAPI struct {
			Key string `yaml:"key"`
			URL string `yaml:"url"`
		} `yaml:"microsoftFaceAPI"`
		GoogleCloudVisionAPI struct {
			Key string `yaml:"key"`
		} `yaml:"googleCloudVisionAPI"`
		AWSRekognition struct {
			Region string `yaml:"region"`
		} `yaml:"awsRekognition"`
	} `yaml:"faceapi"`
	Cache struct {
		Active   bool   `yaml:"active"`
		Provider string `yaml:"provider"`
		InMemory struct {
			Size int `yaml:"size"`
		} `yaml:"inmemory"`
		Redis struct {
			Host string `yaml:"host"`
			Port int    `yaml:"port"`
			DB   int    `yaml:"db"`
		} `yaml:"redis"`
	} `yaml:"cache"`
}

// GetFetcherConfigKeyValue returns a configuration key value of a fetcher
func (cfg *Config) GetFetcherConfigKeyValue(fetcherType string, key string) interface{} {
	for _, v := range cfg.Fetchers {
		if typeValue, ok := v["type"]; ok {
			if typeValue == fetcherType {
				if v, ok := v[key]; ok {
					return v
				}
			}
		}
	}

	return ""
}

// GetPathConfigByPath returns the path config by the specified path
func (cfg *Config) GetPathConfigByPath(path string) *PathConfig {
	for _, p := range cfg.Paths {
		if p.Path == path {
			return &p
		}
	}

	return nil
}

// LoadConfig loads the config file
func LoadConfig(configFile string) (*Config, error) {
	var data []byte
	var err error
	if data, err = ioutil.ReadFile(configFile); err != nil {
		return nil, err
	}

	var c Config
	if err := yaml.Unmarshal([]byte(data), &c); err != nil {
		return nil, err
	}

	return &c, nil
}

// SetConfig set currently active config
func SetConfig(cfg *Config) {
	globalConfig = cfg
}

// GetConfig return the currently active global config
func GetConfig() *Config {
	return globalConfig
}
