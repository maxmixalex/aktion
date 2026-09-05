package main

import (
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"os"
	"strings"

	"github.com/actions/scaleset"
)

type Config struct {
	RegistrationURL string
	MaxRunners      int
	MinRunners      int
	ScaleSetName    string
	Labels          []string
	GitHubApp       scaleset.GitHubAppAuth
	RunnerImage     string
	LogLevel        string
}

func (c *Config) defaults() {
	if c.RunnerImage == "" {
		c.RunnerImage = "ghcr.io/actions/actions-runner:latest"
	}
}

func (c *Config) Validate() error {
	c.defaults()
	if _, err := url.ParseRequestURI(c.RegistrationURL); err != nil {
		return fmt.Errorf("invalid registration URL: %w, it should be the full URL of where you want to register your scale set, e.g. 'https://github.com/org/repo'", err)
	}
	if err := c.GitHubApp.Validate(); err != nil {
		return fmt.Errorf("invalid GitHub App credentials: %w", err)
	}
	if c.ScaleSetName == "" {
		return fmt.Errorf("scale set name is required")
	}
	for i, label := range c.Labels {
		if strings.TrimSpace(label) == "" {
			return fmt.Errorf("label at index %d is empty", i)
		}
	}
	if c.MinRunners < 0 || c.MaxRunners < 0 {
		return fmt.Errorf("min runners and max runners cannot be negative")
	}
	if c.MaxRunners > 0 && c.MaxRunners < c.MinRunners {
		return fmt.Errorf("max runners cannot be less than min runners")
	}
	return nil
}

func (c Config) maxCapacity() int {
	if c.MaxRunners == 0 {
		return math.MaxInt32
	}
	return c.MaxRunners
}

func systemInfo(scaleSetID int) scaleset.SystemInfo {
	return scaleset.SystemInfo{System: "aktion", Subsystem: "listener", CommitSHA: "NA", Version: version, ScaleSetID: scaleSetID}
}

func (c *Config) ScalesetClient() (*scaleset.Client, error) {
	return scaleset.NewClientWithGitHubApp(scaleset.ClientWithGitHubAppConfig{
		GitHubConfigURL: c.RegistrationURL,
		GitHubAppAuth:   c.GitHubApp,
		SystemInfo:      systemInfo(0),
	})
}

func (c *Config) Logger() *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(c.LogLevel) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}

func (c *Config) BuildLabels() []scaleset.Label {
	if len(c.Labels) > 0 {
		labels := make([]scaleset.Label, len(c.Labels))
		for i, name := range c.Labels {
			labels[i] = scaleset.Label{Name: strings.TrimSpace(name)}
		}
		return labels
	}
	return []scaleset.Label{{Name: c.ScaleSetName}}
}
