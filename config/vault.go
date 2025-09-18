package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	Vault  VaultConfig  `mapstructure:"vault"`
	GCP    GCPConfig    `mapstructure:"gcp"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Host string `mapstructure:"host"`
}

type VaultConfig struct {
	Address    string `mapstructure:"address"`
	Token      string `mapstructure:"token"`
	Namespace  string `mapstructure:"namespace"`
	SkipVerify bool   `mapstructure:"skip_verify"`
}

type GCPConfig struct {
	ProjectID              string `mapstructure:"project_id"`
	ServiceAccountPath     string `mapstructure:"service_account_path"`
	DefaultTokenScopes     string `mapstructure:"default_token_scopes"`
	DefaultTTL             string `mapstructure:"default_ttl"`
	MaxTTL                 string `mapstructure:"max_ttl"`
	DisableAutomatedRotation bool `mapstructure:"disable_automated_rotation"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	// Set defaults
	setDefaults()

	// Allow environment variable overrides
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read config file (optional)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found; rely on defaults and env vars
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.host", "0.0.0.0")

	// Vault defaults
	viper.SetDefault("vault.address", "http://127.0.0.1:8200")
	viper.SetDefault("vault.skip_verify", false)

	// GCP defaults
	viper.SetDefault("gcp.default_token_scopes", "https://www.googleapis.com/auth/cloud-platform")
	viper.SetDefault("gcp.default_ttl", "3600s")
	viper.SetDefault("gcp.max_ttl", "7200s")
	viper.SetDefault("gcp.disable_automated_rotation", false)
}
