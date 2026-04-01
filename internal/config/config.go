// Package config handles application configuration via environment variables.
package config

import (
	envconfig "github.com/kelseyhightower/envconfig"
)

// Config holds all application configuration populated from environment variables.
type Config struct {
	Server struct {
		Port int    `envconfig:"PORT" default:"3000"`
		Host string `envconfig:"HOST" default:"localhost"`
	}

	App struct {
		Dev     bool   `envconfig:"DEV" default:"false"`
		RootDir string `envconfig:"ROOT_DIR" default:"."`
	}

	Logging struct {
		Dir string `envconfig:"LOG_DIR" default:"./logs"`
	}
}

// Load reads environment variables into a Config using envconfig.
func Load() (*Config, error) {
	var configuration Config
	err := envconfig.Process("", &configuration)

	return &configuration, err
}
