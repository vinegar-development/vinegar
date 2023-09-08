package config

import (
	_ "embed"
	"errors"
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/vinegarhq/vinegar/internal/dirs"
	"github.com/vinegarhq/vinegar/roblox"
)

//go:embed config.toml
var DefaultConfig string
var Path = filepath.Join(dirs.Config, "config.toml")

type Environment map[string]string

type Application struct {
	Channel        string        `toml:"channel"`
	Renderer       string        `toml:"renderer"`
	ForcedVersion  string        `toml:"forced_version"`
	AutoKillPrefix bool          `toml:"auto_kill_prefix"`
	Dxvk           bool          `toml:"dxvk"`
	FFlags         roblox.FFlags `toml:"fflags"`
	Env            Environment   `toml:"env"`
}

type Config struct {
	Launcher          string      `toml:"launcher"`
	WineRoot          string      `toml:"wineroot"`
	DxvkURL           string      `toml:"dxvk_url"`
	MultipleInstances bool        `toml:"multiple_instances"`
	SanitizeEnv       bool        `toml:"sanitize_env"`
	Player            Application `toml:"player"`
	Studio            Application `toml:"studio"`
	Env               Environment `toml:"env"`
}

func Load() Config {
	var cfg Config

	// This is a default hard-coded configuration. It should not fail.
	_, _ = toml.Decode(DefaultConfig, &cfg)

	if _, err := os.Stat(Path); errors.Is(err, os.ErrNotExist) {
		log.Println("Using default configuration")

		return cfg
	}

	if _, err := toml.DecodeFile(Path, &cfg); err != nil {
		log.Printf("Failed to load configuration: %s, using default configuration", err)
	}

	if cfg.WineRoot != "" {
		log.Printf("Using Wine Root: %s", cfg.WineRoot)
		bin := filepath.Join(cfg.WineRoot, "bin")

		if !filepath.IsAbs(cfg.WineRoot) {
			log.Fatal("ensure that the wine root given is an absolute path")
		}

		_, err := os.Stat(filepath.Join(bin, "wine"))
		if err != nil {
			log.Fatalf("invalid wine root given: %s", err)
		}

		cfg.Env["PATH"] = bin + ":" + os.Getenv("PATH")
	}

	return cfg
}

func (e *Environment) Setenv() {
	for name, value := range *e {
		os.Setenv(name, value)
	}
}

func (c *Config) Print() error {
	return toml.NewEncoder(os.Stdout).Encode(c)
}
