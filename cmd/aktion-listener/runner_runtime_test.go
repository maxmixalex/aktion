package main

import (
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/zyf722/aktion/internal/runnerfeatures"
)

func TestRunnerFeatureRuntimeContainerConfigWrapsEntrypoints(t *testing.T) {
	cfg := runnerFeatureRuntimeContainerConfig(runnerfeatures.RuntimeConfig{
		Entrypoints: []string{"/first.sh", "/second.sh"},
	}, &container.Config{Cmd: []string{"/home/runner/run.sh"}})
	if len(cfg.Entrypoint) != 4 || cfg.Entrypoint[0] != "/bin/sh" || cfg.Entrypoint[1] != "-c" {
		t.Fatalf("unexpected entrypoint: %#v", cfg.Entrypoint)
	}
	if !strings.Contains(cfg.Entrypoint[2], "/first.sh\n/second.sh\nexec \"$@\"") {
		t.Fatalf("unexpected entrypoint script: %q", cfg.Entrypoint[2])
	}
}

func TestRunnerFeatureRuntimeHostConfig(t *testing.T) {
	runtime := runnerfeatures.RuntimeConfig{
		Init:        true,
		Privileged:  true,
		CapAdd:      []string{"SYS_PTRACE"},
		SecurityOpt: []string{"seccomp=unconfined"},
		Mounts: []runnerfeatures.RuntimeMount{
			{Type: "volume", Source: "cache-${devcontainerId}", Target: "/cache"},
			{Type: "bind", Source: "/var/run/docker.sock", Target: "/var/run/docker-host.sock"},
		},
	}
	host, err := runnerFeatureRuntimeHostConfig(runtime, "zyf722/aktion", "local/aktion-runner:test")
	if err != nil {
		t.Fatal(err)
	}
	if host.Init == nil || !*host.Init || !host.Privileged {
		t.Fatalf("init/privileged not applied: %#v", host)
	}
	if len(host.CapAdd) != 1 || host.CapAdd[0] != "SYS_PTRACE" {
		t.Fatalf("capAdd=%v", host.CapAdd)
	}
	if len(host.SecurityOpt) != 1 || host.SecurityOpt[0] != "seccomp=unconfined" {
		t.Fatalf("securityOpt=%v", host.SecurityOpt)
	}
	if len(host.Mounts) != 2 {
		t.Fatalf("mounts=%v", host.Mounts)
	}
	if host.Mounts[0].Type != mount.TypeVolume || strings.Contains(host.Mounts[0].Source, "${devcontainerId}") {
		t.Fatalf("volume mount not resolved: %#v", host.Mounts[0])
	}
	if host.Mounts[1].Type != mount.TypeBind {
		t.Fatalf("bind mount not preserved: %#v", host.Mounts[1])
	}
}

func TestRunnerDevcontainerIDIsStableAndAlphanumeric(t *testing.T) {
	a := runnerDevcontainerID("zyf722/aktion", "image:1")
	b := runnerDevcontainerID("zyf722/aktion", "image:1")
	if a != b || len(a) != 64 {
		t.Fatalf("unexpected id: %q %q", a, b)
	}
	for _, r := range a {
		if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f') {
			t.Fatalf("non-alphanumeric devcontainer id: %q", a)
		}
	}
}
