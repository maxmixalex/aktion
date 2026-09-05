package main

import (
	"context"
	"testing"

	"github.com/actions/scaleset"
)

type fakeRunnerScaleSetClient struct {
	existing    *scaleset.RunnerScaleSet
	getErr      error
	createErr   error
	createCalls int
	createdWith *scaleset.RunnerScaleSet
}

func (f *fakeRunnerScaleSetClient) GetRunnerScaleSet(context.Context, int, string) (*scaleset.RunnerScaleSet, error) {
	return f.existing, f.getErr
}

func (f *fakeRunnerScaleSetClient) CreateRunnerScaleSet(_ context.Context, set *scaleset.RunnerScaleSet) (*scaleset.RunnerScaleSet, error) {
	f.createCalls++
	f.createdWith = set
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &scaleset.RunnerScaleSet{
		ID:            42,
		Name:          set.Name,
		RunnerGroupID: set.RunnerGroupID,
		Labels:        set.Labels,
		RunnerSetting: set.RunnerSetting,
	}, nil
}

func TestGetOrCreateRunnerScaleSetReusesExisting(t *testing.T) {
	existing := &scaleset.RunnerScaleSet{ID: 7, Name: "home-docker", RunnerGroupID: 1}
	client := &fakeRunnerScaleSetClient{existing: existing}

	got, reused, err := getOrCreateRunnerScaleSet(
		context.Background(),
		client,
		"home-docker",
		[]scaleset.Label{{Name: "home-docker"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reused {
		t.Fatal("expected existing scale set to be reused")
	}
	if got.ID != existing.ID {
		t.Fatalf("got scale set id %d, want %d", got.ID, existing.ID)
	}
	if client.createCalls != 0 {
		t.Fatalf("CreateRunnerScaleSet called %d times, want 0", client.createCalls)
	}
}

func TestGetOrCreateRunnerScaleSetCreatesWhenMissing(t *testing.T) {
	client := &fakeRunnerScaleSetClient{}

	got, reused, err := getOrCreateRunnerScaleSet(
		context.Background(),
		client,
		"home-docker",
		[]scaleset.Label{{Name: "home-docker"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if reused {
		t.Fatal("did not expect a missing scale set to be reused")
	}
	if got.ID != 42 {
		t.Fatalf("got scale set id %d, want 42", got.ID)
	}
	if client.createCalls != 1 {
		t.Fatalf("CreateRunnerScaleSet called %d times, want 1", client.createCalls)
	}
	if !client.createdWith.RunnerSetting.DisableUpdate {
		t.Fatal("created scale set must disable runner auto-update")
	}
}
