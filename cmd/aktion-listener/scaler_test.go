package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/actions/scaleset"
	dockerclient "github.com/docker/docker/client"
)

func testScaler(t *testing.T, handler http.HandlerFunc) *Scaler {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := dockerclient.NewClientWithOpts(dockerclient.WithHost(server.URL), dockerclient.WithVersion("1.47"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return &Scaler{
		dockerClient: client, logger: discardLogger(),
		runners: runnerState{idle: map[string]string{}, busy: map[string]string{}},
	}
}

func TestRunnerEventsAreIdempotent(t *testing.T) {
	var removals atomic.Int32
	a := testScaler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || !strings.HasSuffix(r.URL.Path, "/containers/container-id") {
			t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.Path)
		}
		removals.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})
	ctx := context.Background()
	a.runners.addIdle("runner", "container-id")
	for range 2 {
		if err := a.HandleJobStarted(ctx, &scaleset.JobStarted{RunnerName: "runner"}); err != nil {
			t.Fatal(err)
		}
	}
	if a.runners.count() != 1 {
		t.Fatal("duplicate start changed runner count")
	}
	for _, name := range []string{"old-session", "runner", "runner"} {
		if err := a.HandleJobCompleted(ctx, &scaleset.JobCompleted{RunnerName: name}); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.HandleJobStarted(ctx, &scaleset.JobStarted{RunnerName: "runner"}); err != nil {
		t.Fatal(err)
	}
	if a.runners.count() != 0 || removals.Load() != 1 {
		t.Fatalf("count=%d, removals=%d", a.runners.count(), removals.Load())
	}
}

func TestCompletionRetainsRunnerUntilRemovalSucceeds(t *testing.T) {
	for _, success := range []int{http.StatusNoContent, http.StatusNotFound} {
		t.Run(http.StatusText(success), func(t *testing.T) {
			var calls atomic.Int32
			a := testScaler(t, func(w http.ResponseWriter, r *http.Request) {
				call := calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				if call == 1 {
					w.WriteHeader(http.StatusInternalServerError)
				} else {
					w.WriteHeader(success)
				}
				if call == 1 || success == http.StatusNotFound {
					_, _ = w.Write([]byte(`{"message":"remove failed"}`))
				}
			})
			a.runners.addIdle("runner", "container-id")
			a.runners.markBusy("runner")
			job := &scaleset.JobCompleted{RunnerName: "runner"}
			if err := a.HandleJobCompleted(context.Background(), job); err == nil {
				t.Fatal("expected removal error")
			}
			if a.runners.count() != 1 {
				t.Fatal("lost runner after removal failure")
			}
			if err := a.HandleJobCompleted(context.Background(), job); err != nil {
				t.Fatal(err)
			}
			if a.runners.count() != 0 || calls.Load() != 2 {
				t.Fatalf("count=%d, calls=%d", a.runners.count(), calls.Load())
			}
		})
	}
}

func TestFailedStartCleansUpContainerAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var removed atomic.Bool
	a := testScaler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			cancel()
			w.WriteHeader(http.StatusInternalServerError)
		case http.MethodDelete:
			removed.Store(true)
			if r.URL.Query().Get("force") != "1" {
				t.Error("cleanup must force removal")
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s", r.Method)
		}
	})
	if err := a.startRunnerContainer(ctx, "container-id"); err == nil {
		t.Fatal("expected start error")
	}
	if !removed.Load() {
		t.Fatal("container leaked after failed start")
	}
}
