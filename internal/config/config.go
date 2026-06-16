package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	RefreshInterval    string          `yaml:"refresh_interval"` // "1s" | "2s" | "5s"
	ConfirmDestructive bool            `yaml:"confirm_destructive"`
	SortWithinGroup    string          `yaml:"sort_within_group"` // name|cpu|mem|status
	ShowStopped        bool            `yaml:"show_stopped"`
	Theme              string          `yaml:"theme"` // dark|light|mono
	CardFields         []string        `yaml:"card_fields"`
	GroupOrder         []string        `yaml:"group_order"`
	GroupCollapsed     map[string]bool `yaml:"group_collapsed"`
}

func Default() Config {
	return Config{
		RefreshInterval:    "2s",
		ConfirmDestructive: true,
		SortWithinGroup:    "name",
		ShowStopped:        true,
		Theme:              "dark",
		CardFields:         []string{"status", "health", "cpu", "mem", "net", "port"},
		GroupOrder:         []string{},
		GroupCollapsed:     map[string]bool{},
	}
}

// Interval parses RefreshInterval, falling back to 2s.
func (c Config) Interval() time.Duration {
	d, err := time.ParseDuration(c.RefreshInterval)
	if err != nil || d <= 0 {
		return 2 * time.Second
	}
	return d
}

// Path returns the default config path honoring XDG_CONFIG_HOME.
func Path() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "ekiben", "config.yml")
}

// Load reads the config, writing defaults if the file is missing. On parse
// error it returns defaults (callers may surface a warning).
func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		c := Default()
		return c, c.Save(path)
	}
	if err != nil {
		return Default(), err
	}
	c := Default()
	if err := yaml.Unmarshal(b, &c); err != nil {
		return Default(), fmt.Errorf("parse config %s: %w", path, err)
	}
	return c, nil
}

func (c Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
