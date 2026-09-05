package main

import (
	"context"
	"fmt"

	"github.com/actions/scaleset"
)

type runnerScaleSetClient interface {
	GetRunnerScaleSet(context.Context, int, string) (*scaleset.RunnerScaleSet, error)
	CreateRunnerScaleSet(context.Context, *scaleset.RunnerScaleSet) (*scaleset.RunnerScaleSet, error)
}

const defaultRunnerGroupID = 1

func getOrCreateRunnerScaleSet(
	ctx context.Context,
	client runnerScaleSetClient,
	name string,
	labels []scaleset.Label,
) (*scaleset.RunnerScaleSet, bool, error) {
	existing, err := client.GetRunnerScaleSet(ctx, defaultRunnerGroupID, name)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get runner scale set: %w", err)
	}
	if existing != nil {
		return existing, true, nil
	}

	created, err := client.CreateRunnerScaleSet(ctx, &scaleset.RunnerScaleSet{
		Name:          name,
		RunnerGroupID: defaultRunnerGroupID,
		Labels:        labels,
		RunnerSetting: scaleset.RunnerSetting{DisableUpdate: true},
	})
	if err != nil {
		return nil, false, fmt.Errorf("failed to create runner scale set: %w", err)
	}
	return created, false, nil
}
