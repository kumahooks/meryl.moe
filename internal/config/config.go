// Package config handles application configuration via environment variables.
package config

import "github.com/kelseyhightower/envconfig"

type Config struct {
	Server struct {
		Port int    `envconfig:"PORT" default:"3000"`
		Host string `envconfig:"HOST" default:"localhost"`
	}

	Logging struct {
		FilePath string `envconfig:"LOG_FILE" default:"./logs/app.log"`
	}
}

func Load() (*Config, error) {
	var configuration Config
	err := envconfig.Process("", &configuration)

	return &configuration, err
}
