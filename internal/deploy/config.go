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

// ExampleConfig returns an example configuration for documentation.
func ExampleConfig() string {
	return `# deploy.toml - Deployment configuration
# Place this file in your project root.

[[environments]]
name = "local"
target = ""
path = "./deploy"
keepN = 3
baseURL = "http://localhost:1314"

[[environments]]
name = "prod"
target = "user@host"
path = "/var/www/site"
keepN = 5
baseURL = "https://example.com"
`
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
	return &config, nil
}

// LoadConfig loads configuration from deploy.toml.
// Returns an error if the config file is not found.
func LoadConfig(configPath string) (*Config, error) {
	path := defaultConfigPath(configPath)
	data, err := os.ReadFile(path)
	if err != nil {
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
