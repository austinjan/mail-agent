// Package config loads mail-agent's YAML config file.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type IMAPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Folder   string `yaml:"folder"`
}

type DefaultsConfig struct {
	Since string `yaml:"since"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type AttachmentsConfig struct {
	Dir string `yaml:"dir"`
}

type Config struct {
	IMAP        IMAPConfig        `yaml:"imap"`
	Defaults    DefaultsConfig    `yaml:"defaults"`
	Database    DatabaseConfig    `yaml:"database"`
	Attachments AttachmentsConfig `yaml:"attachments"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	return &cfg, nil
}
