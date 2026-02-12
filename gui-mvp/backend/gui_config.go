package main

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type GuiConfig struct {
	Version int `yaml:"version"`

	Envs struct {
		Pinned []struct {
			Name string `yaml:"name"`
			Path string `yaml:"path"`
		} `yaml:"pinned"`

		Scan []struct {
			Root    string `yaml:"root"`
			Depth   int    `yaml:"depth"`
			Pattern string `yaml:"pattern"`
		} `yaml:"scan"`
	} `yaml:"envs"`

	Server struct {
		Addr string `yaml:"addr"`
	} `yaml:"server"`
}

func defaultGuiConfig() GuiConfig {
	var c GuiConfig
	c.Version = 1

	home, _ := os.UserHomeDir()

	c.Envs.Scan = []struct {
		Root    string `yaml:"root"`
		Depth   int    `yaml:"depth"`
		Pattern string `yaml:"pattern"`
	}{
		{Root: filepath.Join(home, "lyenv-envs"), Depth: 3, Pattern: "lyenv.yaml"},
		{Root: filepath.Join(home, "projects"), Depth: 3, Pattern: "lyenv.yaml"},
	}

	c.Server.Addr = "127.0.0.1:18888"
	return c
}

func loadGuiConfig(path string) (GuiConfig, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaultGuiConfig(), false, nil
		}
		return GuiConfig{}, false, err
	}
	var c GuiConfig
	if err := yaml.Unmarshal(b, &c); err != nil {
		return GuiConfig{}, true, err
	}
	if c.Version == 0 {
		c.Version = 1
	}
	return c, true, nil
}

func saveGuiConfig(path string, c GuiConfig) error {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	out, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}
