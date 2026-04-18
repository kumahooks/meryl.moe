// Package config handles application configuration via environment variables.
package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all application configuration populated from environment variables.
type Config struct {
	Server struct {
		Port int    `envconfig:"PORT" default:"3000"`
		Host string `envconfig:"HOST" default:"127.0.0.1"`
	}

	App struct {
		Dev     bool   `envconfig:"DEV" default:"true"`
		RootDir string `envconfig:"ROOT_DIR" default:"."`
	}

	Logging struct {
		Dir string `envconfig:"LOG_DIR" default:"./logs"`
	}

	DB struct {
		CorePath   string `envconfig:"DB_PATH" default:"./data/meryl.db"`
		WorkerPath string `envconfig:"WORKER_DB_PATH" default:"./data/worker.db"`
	}

	Session struct {
		TTL time.Duration `envconfig:"SESSION_TTL" default:"168h"`
	}

	Kipple struct {
		Dir string `envconfig:"KIPPLE_DIR" default:"./data/kipple"`
		// Quota is the per-user storage cap in bytes. Default: 3 GiB (3 * 1024^3).
		Quota int64 `envconfig:"KIPPLE_QUOTA" default:"3221225472"`
	}
}

// Load reads environment variables into a Config using envconfig.
func Load() (*Config, error) {
	var configuration Config
	err := envconfig.Process("", &configuration)

	return &configuration, err
}
