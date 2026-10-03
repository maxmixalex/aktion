package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/image"
)

type runnerImageClient interface {
	ImageInspectWithRaw(context.Context, string) (image.InspectResponse, []byte, error)
	ImagePull(context.Context, string, image.PullOptions) (io.ReadCloser, error)
}

func ensureRunnerImage(ctx context.Context, client runnerImageClient, imageName string, logger *slog.Logger) error {
	if _, _, err := client.ImageInspectWithRaw(ctx, imageName); err == nil {
		logger.Info("Using local runner image", slog.String("image", imageName))
		return nil
	} else if !cerrdefs.IsNotFound(err) {
		return fmt.Errorf("failed to inspect runner image %q: %w", imageName, err)
	}

	logger.Info("Runner image not found locally; pulling", slog.String("image", imageName))
	pull, err := client.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull runner image %q: %w", imageName, err)
	}
	defer pull.Close()
	decoder := json.NewDecoder(pull)
	for {
		var progress struct {
			Error       string `json:"error"`
			ErrorDetail struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
		}
		if err := decoder.Decode(&progress); err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("failed to read runner image pull response for %q: %w", imageName, err)
		}
		if progress.ErrorDetail.Message != "" {
			progress.Error = progress.ErrorDetail.Message
		}
		if progress.Error != "" {
			return fmt.Errorf("failed to pull runner image %q: %s", imageName, progress.Error)
		}
	}
	return nil
}
