// internal/config/config.go
package config

import (
	"os"

	"gslb/internal/models"

	"gopkg.in/yaml.v2"
)

// Load reads the YAML configuration from the given path and unmarshals it into a Configuration struct.
func Load(path string) (models.Configuration, error) {
	var cfg models.Configuration

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return cfg, err
	}

	return cfg, nil
}
