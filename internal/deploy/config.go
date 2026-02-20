package deploy

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config represents the deploy.toml configuration file.
type Config struct {
	Environments []Environment `toml:"environments"`
}

// DefaultEnvironments returns the hardcoded default environments.
// Used when deploy.toml is missing or doesn't define an environment.
func DefaultEnvironments() []Environment {
	return []Environment{
		{
			Name:    "local",
			Target:  "",
			Path:    "./deploy",
			KeepN:   3,
			BaseURL: "http://localhost:1314",
		},
		{
			Name:    "prod",
			Target:  "root@45.77.6.158",
			Path:    "/var/www/juniperbible",
			KeepN:   5,
			BaseURL: "https://juniperbible.org",
		},
	}
}

// defaultConfigPath returns the default config path if empty
func defaultConfigPath(configPath string) string {
	if configPath == "" {
		return "deploy.toml"
	}
	return configPath
}

// parseConfigFile parses the TOML config file
func parseConfigFile(data []byte) (*Config, error) {
	var config Config
	if _, err := toml.Decode(string(data), &config); err != nil {
		return nil, err
	}
	if len(config.Environments) == 0 {
		config.Environments = DefaultEnvironments()
	}
	return &config, nil
}

// LoadConfig loads configuration from deploy.toml if present,
// falling back to defaults for missing environments.
func LoadConfig(configPath string) (*Config, error) {
	path := defaultConfigPath(configPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Environments: DefaultEnvironments()}, nil
		}
		return nil, err
	}
	return parseConfigFile(data)
}

// GetEnvironment returns the environment configuration for the given name.
// Returns an error if the environment is not found.
func (c *Config) GetEnvironment(name string) (Environment, bool) {
	for _, env := range c.Environments {
		if env.Name == name {
			return env, true
		}
	}
	return Environment{}, false
}

// FindConfigFile searches for deploy.toml in the current directory and parent directories.
func FindConfigFile() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		path := filepath.Join(dir, "deploy.toml")
		if _, err := os.Stat(path); err == nil {
			return path
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}
