package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/image"
)

type fakeRunnerImageClient struct {
	inspectErr error
	pullErr    error
	pullCalls  int
}

func (f *fakeRunnerImageClient) ImageInspectWithRaw(context.Context, string) (image.InspectResponse, []byte, error) {
	return image.InspectResponse{}, nil, f.inspectErr
}

func (f *fakeRunnerImageClient) ImagePull(context.Context, string, image.PullOptions) (io.ReadCloser, error) {
	f.pullCalls++
	if f.pullErr != nil {
		return nil, f.pullErr
	}
	return io.NopCloser(bytes.NewBufferString("{}")), nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestEnsureRunnerImageUsesLocalImage(t *testing.T) {
	client := &fakeRunnerImageClient{}
	if err := ensureRunnerImage(context.Background(), client, "local/aktion-runner:test", discardLogger()); err != nil {
		t.Fatal(err)
	}
	if client.pullCalls != 0 {
		t.Fatalf("pull called %d times, want 0", client.pullCalls)
	}
}

func TestEnsureRunnerImagePullsWhenMissing(t *testing.T) {
	client := &fakeRunnerImageClient{inspectErr: cerrdefs.ErrNotFound}
	if err := ensureRunnerImage(context.Background(), client, "ghcr.io/actions/actions-runner:latest", discardLogger()); err != nil {
		t.Fatal(err)
	}
	if client.pullCalls != 1 {
		t.Fatalf("pull called %d times, want 1", client.pullCalls)
	}
}

func TestEnsureRunnerImageDoesNotPullOnInspectFailure(t *testing.T) {
	client := &fakeRunnerImageClient{inspectErr: errors.New("docker unavailable")}
	if err := ensureRunnerImage(context.Background(), client, "local/aktion-runner:test", discardLogger()); err == nil {
		t.Fatal("expected inspect failure")
	}
	if client.pullCalls != 0 {
		t.Fatalf("pull called %d times, want 0", client.pullCalls)
	}
}
