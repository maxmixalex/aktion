package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/actions/scaleset"
	"github.com/actions/scaleset/listener"
	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/zyf722/aktion/internal/runnerfeatures"
)

type Scaler struct {
	runners        runnerState
	runnerImage    string
	runnerRuntime  runnerfeatures.RuntimeConfig
	scaleSetID     int
	dockerClient   *dockerclient.Client
	scalesetClient *scaleset.Client
	minRunners     int
	maxRunners     int
	logger         *slog.Logger
}

func (a *Scaler) HandleDesiredRunnerCount(ctx context.Context, count int) (int, error) {
	currentCount := a.runners.count()
	targetRunnerCount := min(a.maxRunners, a.minRunners+count)
	a.logger.Info("Scale demand", slog.Int("assignedJobs", count), slog.Int("currentRunners", currentCount), slog.Int("targetRunners", targetRunnerCount))
	switch {
	case targetRunnerCount == currentCount:
		return currentCount, nil
	case targetRunnerCount > currentCount:
		scaleUp := targetRunnerCount - currentCount
		a.logger.Info("Scaling up runners", slog.Int("currentCount", currentCount), slog.Int("desiredCount", targetRunnerCount), slog.Int("scaleUp", scaleUp))
		for range scaleUp {
			if _, err := a.startRunner(ctx); err != nil {
				return 0, fmt.Errorf("failed to start runner: %w", err)
			}
		}
		return a.runners.count(), nil
	}
	return a.runners.count(), nil
}

func (a *Scaler) HandleJobStarted(ctx context.Context, jobInfo *scaleset.JobStarted) error {
	a.logger.Info(
		"Job started",
		slog.Int64("runnerRequestId", jobInfo.RunnerRequestID),
		slog.String("jobId", jobInfo.JobID),
		slog.String("runnerName", jobInfo.RunnerName),
		slog.String("repository", jobInfo.OwnerName+"/"+jobInfo.RepositoryName),
		slog.String("jobDisplayName", jobInfo.JobDisplayName),
		slog.Int64("workflowRunId", jobInfo.WorkflowRunID),
		slog.String("jobWorkflowRef", jobInfo.JobWorkflowRef),
		slog.String("eventName", jobInfo.EventName),
	)
	a.runners.markBusy(jobInfo.RunnerName)
	return nil
}

func (a *Scaler) HandleJobCompleted(ctx context.Context, jobInfo *scaleset.JobCompleted) error {
	a.logger.Info(
		"Job completed",
		slog.Int64("runnerRequestId", jobInfo.RunnerRequestID),
		slog.String("jobId", jobInfo.JobID),
		slog.String("runnerName", jobInfo.RunnerName),
		slog.String("repository", jobInfo.OwnerName+"/"+jobInfo.RepositoryName),
		slog.String("jobDisplayName", jobInfo.JobDisplayName),
		slog.Int64("workflowRunId", jobInfo.WorkflowRunID),
		slog.String("result", jobInfo.Result),
	)
	containerID := a.runners.markDone(jobInfo.RunnerName)
	if err := a.dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("failed to remove runner container: %w", err)
	}
	return nil
}

func runnerProxyEnv() []string {
	keys := []string{"http_proxy", "https_proxy", "no_proxy", "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"}
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func runnerLabels() map[string]string {
	labels := map[string]string{"io.aktion.managed": "true"}
	if v := os.Getenv("AKTION_REPOSITORY"); v != "" {
		labels["io.aktion.repository"] = v
	}
	if v := os.Getenv("AKTION_SERVICE_ID"); v != "" {
		labels["io.aktion.service-id"] = v
	}
	if v := os.Getenv("SCALE_SET_NAME"); v != "" {
		labels["io.aktion.scale-set"] = v
	}
	return labels
}

func (a *Scaler) startRunner(ctx context.Context) (string, error) {
	name := fmt.Sprintf("runner-%s", uuid.NewString()[:8])
	jit, err := a.scalesetClient.GenerateJitRunnerConfig(
		ctx,
		&scaleset.RunnerScaleSetJitRunnerSetting{Name: name},
		a.scaleSetID,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate JIT config: %w", err)
	}

	env := []string{fmt.Sprintf("ACTIONS_RUNNER_INPUT_JITCONFIG=%s", jit.EncodedJITConfig)}
	env = append(env, runnerProxyEnv()...)

	containerConfig := runnerFeatureRuntimeContainerConfig(a.runnerRuntime, &container.Config{
		Image:  a.runnerImage,
		User:   "runner",
		Cmd:    []string{"/home/runner/run.sh"},
		Env:    env,
		Labels: runnerLabels(),
	})
	hostConfig, err := runnerFeatureRuntimeHostConfig(a.runnerRuntime, os.Getenv("AKTION_REPOSITORY"), a.runnerImage)
	if err != nil {
		return "", fmt.Errorf("failed to apply runner feature runtime metadata: %w", err)
	}
	c, err := a.dockerClient.ContainerCreate(
		ctx,
		containerConfig,
		hostConfig,
		nil, nil,
		name,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create runner container: %w", err)
	}
	if err := a.dockerClient.ContainerStart(ctx, c.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start runner: %w", err)
	}
	a.runners.addIdle(name, c.ID)
	a.logger.Info("Runner provisioned", slog.String("runnerName", name), slog.String("repository", os.Getenv("AKTION_REPOSITORY")))
	return name, nil
}

func (a *Scaler) shutdown(ctx context.Context) {
	a.logger.Info("Shutting down runners")
	a.runners.mu.Lock()
	defer a.runners.mu.Unlock()
	for name, containerID := range a.runners.idle {
		a.logger.Info("Removing idle runner", slog.String("name", name), slog.String("containerID", containerID))
		if err := a.dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
			a.logger.Error("Failed to remove idle runner container", slog.String("name", name), slog.String("containerID", containerID), slog.String("error", err.Error()))
		}
	}
	clear(a.runners.idle)
	for name, containerID := range a.runners.busy {
		a.logger.Info("Removing busy runner", slog.String("name", name), slog.String("containerID", containerID))
		if err := a.dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
			a.logger.Error("Failed to remove busy runner container", slog.String("name", name), slog.String("containerID", containerID), slog.String("error", err.Error()))
		}
	}
	clear(a.runners.busy)
}

var _ listener.Scaler = (*Scaler)(nil)

type runnerState struct {
	mu   sync.Mutex
	idle map[string]string
	busy map[string]string
}

func (r *runnerState) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.idle) + len(r.busy)
}

func (r *runnerState) markBusy(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, ok := r.idle[name]
	if !ok {
		panic("marking non-existent runner busy")
	}
	delete(r.idle, name)
	r.busy[name] = state
}

func (r *runnerState) markDone(name string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.markDoneUnlocked(name)
}

func (r *runnerState) markDoneUnlocked(name string) string {
	if containerID, ok := r.busy[name]; ok {
		delete(r.busy, name)
		return containerID
	}
	if containerID, ok := r.idle[name]; ok {
		delete(r.idle, name)
		return containerID
	}
	panic("marking non-existent runner done")
}

func (r *runnerState) addIdle(name, containerID string) {
	r.mu.Lock()
	r.idle[name] = containerID
	r.mu.Unlock()
}
