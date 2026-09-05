package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/strslice"
	dockerclient "github.com/docker/docker/client"
	"github.com/zyf722/aktion/internal/runnerfeatures"
)

func loadRunnerFeatureRuntime(ctx context.Context, client *dockerclient.Client, imageName string) (runnerfeatures.RuntimeConfig, error) {
	inspect, _, err := client.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		return runnerfeatures.RuntimeConfig{}, fmt.Errorf("inspect runner image runtime metadata: %w", err)
	}
	if inspect.Config == nil || inspect.Config.Labels == nil {
		return runnerfeatures.RuntimeConfig{}, nil
	}
	return runnerfeatures.DecodeRuntime(inspect.Config.Labels[runnerfeatures.RuntimeLabel])
}

func runnerFeatureRuntimeContainerConfig(runtime runnerfeatures.RuntimeConfig, base *container.Config) *container.Config {
	if len(runtime.Entrypoints) == 0 {
		return base
	}
	script := "set -e\n" + strings.Join(runtime.Entrypoints, "\n") + "\nexec \"$@\""
	base.Entrypoint = strslice.StrSlice{"/bin/sh", "-c", script, "-"}
	return base
}

func runnerFeatureRuntimeHostConfig(runtime runnerfeatures.RuntimeConfig, repository, runnerImage string) (*container.HostConfig, error) {
	host := &container.HostConfig{
		ExtraHosts: []string{"host.docker.internal:host-gateway"},
		Privileged: runtime.Privileged,
		CapAdd:     strslice.StrSlice(append([]string(nil), runtime.CapAdd...)),
		SecurityOpt: append([]string(nil), runtime.SecurityOpt...),
	}
	if runtime.Init {
		value := true
		host.Init = &value
	}

	devcontainerID := runnerDevcontainerID(repository, runnerImage)
	for _, m := range runtime.Mounts {
		source := strings.ReplaceAll(m.Source, "${devcontainerId}", devcontainerID)
		target := strings.ReplaceAll(m.Target, "${devcontainerId}", devcontainerID)
		if target == "" {
			return nil, fmt.Errorf("runner feature mount has empty target")
		}
		var mountType mount.Type
		switch m.Type {
		case "bind":
			mountType = mount.TypeBind
			if source == "" {
				return nil, fmt.Errorf("bind mount %q has empty source", target)
			}
		case "volume", "":
			mountType = mount.TypeVolume
		case "tmpfs":
			mountType = mount.TypeTmpfs
		default:
			return nil, fmt.Errorf("unsupported runner feature mount type %q for %s", m.Type, target)
		}
		host.Mounts = append(host.Mounts, mount.Mount{
			Type:   mountType,
			Source: source,
			Target: target,
		})
	}
	return host, nil
}

func runnerDevcontainerID(repository, runnerImage string) string {
	sum := sha256.Sum256([]byte(repository + "\x00" + runnerImage))
	return hex.EncodeToString(sum[:])
}
