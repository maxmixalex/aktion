package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/actions/scaleset"
	"github.com/actions/scaleset/listener"
	dockerclient "github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func init() {
	flags := cmd.Flags()
	flags.StringVar(&cfg.RegistrationURL, "url", "", "REQUIRED: URL where to register your scale set (e.g. https://github.com/org/repo)")
	flags.IntVar(&cfg.MaxRunners, "max-runners", 10, "Maximum number of runners")
	flags.IntVar(&cfg.MinRunners, "min-runners", 0, "Minimum number of runners")
	flags.StringVar(&cfg.ScaleSetName, "name", "", "REQUIRED: Name of your scale set")
	flags.StringSliceVar(&cfg.Labels, "labels", nil, "Labels for workflow targeting (comma-separated or repeated). Defaults to --name if not provided.")
	flags.StringVar(&cfg.RunnerGroup, "runner-group", scaleset.DefaultRunnerGroup, "Name of the runner group your scale set should belong to")
	flags.StringVar(&cfg.RunnerImage, "runner-image", "ghcr.io/actions/actions-runner:latest", "Linux runner container image")
	flags.StringVar(&cfg.GitHubApp.ClientID, "app-client-id", "", "GitHub App client id")
	flags.Int64Var(&cfg.GitHubApp.InstallationID, "app-installation-id", 0, "GitHub App installation ID")
	flags.StringVar(&cfg.GitHubApp.PrivateKey, "app-private-key", "", "GitHub App private key")
	flags.StringVar(&cfg.Token, "token", "", "Personal access token (can be used in place of a GitHub App, although not recommended)")
	flags.StringVar(&cfg.LogLevel, "log-level", "info", "Logging level (debug, info, warn, error)")
	flags.StringVar(&cfg.LogFormat, "log-format", "text", "Logging format (text, json). If invalid value is provided, defaults to no logs.")
	_ = cmd.MarkFlagRequired("url")
	_ = cmd.MarkFlagRequired("name")
}

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func runListener(ctx context.Context, c Config) error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}
	logger := c.Logger()
	scalesetClient, err := c.ScalesetClient()
	if err != nil {
		return fmt.Errorf("failed to create scaleset client: %w", err)
	}
	var runnerGroupID int
	switch c.RunnerGroup {
	case scaleset.DefaultRunnerGroup:
		runnerGroupID = 1
	default:
		runnerGroup, err := scalesetClient.GetRunnerGroupByName(ctx, c.RunnerGroup)
		if err != nil {
			return fmt.Errorf("failed to get runner group ID: %w", err)
		}
		runnerGroupID = runnerGroup.ID
	}
	scaleSet, reused, err := getOrCreateRunnerScaleSet(ctx, scalesetClient, runnerGroupID, c.ScaleSetName, c.BuildLabels())
	if err != nil {
		return err
	}
	if reused {
		logger.Info("Reusing existing runner scale set", slog.Int("scaleSetID", scaleSet.ID), slog.String("scaleSetName", scaleSet.Name))
	} else {
		logger.Info("Created runner scale set", slog.Int("scaleSetID", scaleSet.ID), slog.String("scaleSetName", scaleSet.Name))
	}
	scalesetClient.SetSystemInfo(systemInfo(scaleSet.ID))
	defer func() {
		logger.Info("Deleting runner scale set", slog.Int("scaleSetID", scaleSet.ID))
		if err := scalesetClient.DeleteRunnerScaleSet(context.WithoutCancel(ctx), scaleSet.ID); err != nil {
			logger.Error("Failed to delete runner scale set", slog.Int("scaleSetID", scaleSet.ID), slog.String("error", err.Error()))
		}
	}()

	dockerClient, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	if err := ensureRunnerImage(ctx, dockerClient, c.RunnerImage, logger); err != nil {
		return err
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = uuid.NewString()
		logger.Info("Failed to get hostname, fallback to uuid", "uuid", hostname, "error", err)
	}
	sessionClient, err := scalesetClient.MessageSessionClient(ctx, scaleSet.ID, hostname)
	if err != nil {
		return fmt.Errorf("failed to create message session client: %w", err)
	}
	defer sessionClient.Close(context.Background())

	logger.Info("Initializing listener")
	l, err := listener.New(sessionClient, listener.Config{
		ScaleSetID: scaleSet.ID,
		MaxRunners: c.MaxRunners,
		Logger: logger.WithGroup("listener"),
	})
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	scaler := &Scaler{
		logger: logger.WithGroup("scaler"),
		runners: runnerState{idle: make(map[string]string), busy: make(map[string]string)},
		runnerImage: c.RunnerImage,
		minRunners: c.MinRunners,
		maxRunners: c.MaxRunners,
		dockerClient: dockerClient,
		scalesetClient: scalesetClient,
		scaleSetID: scaleSet.ID,
	}
	defer scaler.shutdown(context.WithoutCancel(ctx))

	logger.Info("Starting listener")
	if err := l.Run(ctx, scaler); !errors.Is(err, context.Canceled) {
		return fmt.Errorf("listener run failed: %w", err)
	}
	return nil
}

var version = "dev"
var cfg Config

var cmd = &cobra.Command{
	Use: "aktion-listener",
	Short: "Aktion Linux Docker runner scale-set listener",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
		defer cancel()
		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("invalid configuration: %w", err)
		}
		return runListener(ctx, cfg)
	},
}
