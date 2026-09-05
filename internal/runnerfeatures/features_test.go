package runnerfeatures

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestDefaultSelectionsUseOfficialFeatures(t *testing.T) {
	defaults := DefaultSelections()
	for _, ref := range []string{
		"ghcr.io/devcontainers/features/common-utils:2",
		"ghcr.io/devcontainers/features/python:1",
		"ghcr.io/devcontainers/features/git-lfs:1",
		"builtin:build",
		"builtin:native",
	} {
		if _, ok := defaults[ref]; !ok {
			t.Fatalf("default selections missing %q: %#v", ref, defaults)
		}
	}
	for _, removed := range []string{"builtin:network", "builtin:python", "builtin:archives"} {
		if _, ok := defaults[removed]; ok {
			t.Fatalf("legacy builtin %q must not remain in defaults", removed)
		}
	}
}

func TestBuiltinSVNFeature(t *testing.T) {
	catalog, err := Catalog()
	if err != nil {
		t.Fatal(err)
	}
	svn, ok := catalog["svn"]
	if !ok {
		t.Fatalf("svn builtin missing from catalog: %#v", catalog)
	}
	if svn.Manifest.Name != "Subversion client" {
		t.Fatalf("svn name=%q", svn.Manifest.Name)
	}
	install := string(svn.Install)
	if !strings.Contains(install, "apt-get install -y --no-install-recommends subversion") {
		t.Fatalf("svn install script=%q", install)
	}
	pkg, err := loadBuiltin("builtin:svn")
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Manifest.ID != "svn" || len(pkg.Files) != 1 || pkg.Files[0].Path != "install.sh" {
		t.Fatalf("svn package=%#v", pkg)
	}
}

func TestSelectionsDecodeDevContainerStyleValues(t *testing.T) {
	var selections Selections
	data := "{\n" +
		"  \"ghcr.io/devcontainers/features/node:1\": \"24\",\n" +
		"  \"ghcr.io/devcontainers/features/go:1\": {\"version\":\"1.26\"},\n" +
		"  \"ghcr.io/devcontainers/features/rust:1\": true,\n" +
		"  \"ghcr.io/devcontainers/features/php:1\": false\n" +
		"}"
	if err := json.Unmarshal([]byte(data), &selections); err != nil {
		t.Fatal(err)
	}
	if got := selections["ghcr.io/devcontainers/features/node:1"]["version"]; got != "24" {
		t.Fatalf("node shorthand=%v", got)
	}
	if got := selections["ghcr.io/devcontainers/features/go:1"]["version"]; got != "1.26" {
		t.Fatalf("go version=%v", got)
	}
	if _, ok := selections["ghcr.io/devcontainers/features/rust:1"]; !ok {
		t.Fatal("true feature should be included")
	}
	if disabled, ok := selections["ghcr.io/devcontainers/features/php:1"]; !ok || disabled != nil {
		t.Fatalf("false feature should be preserved as a disabled marker: %#v", disabled)
	}
	normalized, err := NormalizeSelections(selections)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := normalized["ghcr.io/devcontainers/features/php:1"]; ok {
		t.Fatal("false feature should be omitted after normalization")
	}
}


func TestSelectionsRejectLegacyArray(t *testing.T) {
	var selections Selections
	err := json.Unmarshal([]byte("[\"build\",\"python\"]"), &selections)
	if err == nil || !strings.Contains(err.Error(), "legacy string array") {
		t.Fatalf("unexpected legacy config error: %v", err)
	}
}

func TestImageNameIsStableWithoutNetwork(t *testing.T) {
	features := Selections{
		"builtin:build": {},
		"ghcr.io/devcontainers/features/node:1": {"version": "24"},
	}
	a, baseA, err := ImageName("", features)
	if err != nil {
		t.Fatal(err)
	}
	b, baseB, err := ImageName("", features)
	if err != nil {
		t.Fatal(err)
	}
	if a != b || baseA != DefaultBaseImage || baseB != DefaultBaseImage {
		t.Fatalf("unstable image plan: %q/%q %q/%q", a, baseA, b, baseB)
	}
	if !strings.HasPrefix(a, "local/aktion-runner:") {
		t.Fatalf("unexpected image %q", a)
	}
}

func TestOCIReferenceParsing(t *testing.T) {
	ref, err := ParseOCIReference("GHCR.IO/devcontainers/features/python:1")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Registry != "ghcr.io" || ref.Repository != "devcontainers/features/python" || ref.Reference != "1" {
		t.Fatalf("unexpected ref: %#v", ref)
	}
	latest, err := ParseOCIReference("ghcr.io/devcontainers/features/go")
	if err != nil {
		t.Fatal(err)
	}
	if latest.Reference != "latest" {
		t.Fatalf("implicit version=%q", latest.Reference)
	}
}

type fakeFetcher map[string]FeaturePackage

func (f fakeFetcher) FetchFeature(_ context.Context, ref string) (FeaturePackage, error) {
	pkg, ok := f[ref]
	if !ok {
		return FeaturePackage{}, context.Canceled
	}
	return pkg, nil
}

func TestResolvePlanOrdersDependenciesAndAppliesOptions(t *testing.T) {
	bRef := "ghcr.io/example/features/b:1"
	aRef := "ghcr.io/example/features/a:1"
	fetcher := fakeFetcher{
		bRef: {
			Reference: bRef,
			Identity:  "ghcr.io/example/features/b",
			Canonical: "ghcr.io/example/features/b@sha256:b",
			ManifestDigest: "sha256:b",
			Manifest: Manifest{
				ID: "b", Version: "1.0.0", Name: "B",
				ContainerEnv: map[string]string{"B_HOME": "/opt/b"},
			},
			Files: []FeatureFile{{Path: "install.sh", Mode: 0755, Data: []byte("#!/bin/sh\n")}},
		},
		aRef: {
			Reference: aRef,
			Identity:  "ghcr.io/example/features/a",
			Canonical: "ghcr.io/example/features/a@sha256:a",
			ManifestDigest: "sha256:a",
			Manifest: Manifest{
				ID: "a", Version: "1.0.0", Name: "A",
				Options: map[string]OptionDefinition{
					"version": {Type: "string", Default: "latest"},
					"debug":   {Type: "boolean", Default: false},
				},
				DependsOn: map[string]json.RawMessage{
					bRef: json.RawMessage("{}"),
				},
				Init: true,
				CapAdd: []string{"SYS_PTRACE"},
				SecurityOpt: []string{"seccomp=unconfined"},
			},
			Files: []FeatureFile{{Path: "install.sh", Mode: 0755, Data: []byte("#!/bin/sh\n")}},
		},
	}
	resolver := &Resolver{OCI: fetcher}
	plan, err := resolver.ResolvePlan(context.Background(), "example/base:1", Selections{
		aRef: {"version": "2.0", "debug": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Features) != 2 || plan.Features[0].Manifest.ID != "b" || plan.Features[1].Manifest.ID != "a" {
		t.Fatalf("dependency order=%v", featureIDs(plan.Features))
	}
	if !plan.Runtime.Init || len(plan.Runtime.CapAdd) != 1 || plan.Runtime.CapAdd[0] != "SYS_PTRACE" {
		t.Fatalf("runtime=%#v", plan.Runtime)
	}
	for _, want := range []string{
		"VERSION='2.0'",
		"DEBUG='true'",
		"ENV B_HOME=\"/opt/b\"",
		"USER runner",
		RuntimeLabel,
	} {
		if !strings.Contains(plan.Dockerfile, want) {
			t.Fatalf("Dockerfile missing %q:\n%s", want, plan.Dockerfile)
		}
	}
}

func TestResolvePlanRejectsUnknownOption(t *testing.T) {
	ref := "ghcr.io/example/features/a:1"
	resolver := &Resolver{OCI: fakeFetcher{
		ref: {
			Reference: ref,
			Identity: "ghcr.io/example/features/a",
			Canonical: "ghcr.io/example/features/a@sha256:a",
			Manifest: Manifest{ID: "a", Version: "1.0.0", Name: "A"},
			Files: []FeatureFile{{Path: "install.sh", Data: []byte("#!/bin/sh\n")}},
		},
	}}
	if _, err := resolver.ResolvePlan(context.Background(), "base:1", Selections{ref: {"mystery": true}}); err == nil {
		t.Fatal("expected unknown option error")
	}
}

func TestEmptySelectionsUseBaseImageDirectly(t *testing.T) {
	plan, err := NewResolver("").ResolvePlan(context.Background(), "example/base:1", Selections{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Image != "example/base:1" || plan.Dockerfile != "" || len(plan.Features) != 0 {
		t.Fatalf("unexpected empty plan: %#v", plan)
	}
}

func TestExtractFeatureArchive(t *testing.T) {
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	tw := tar.NewWriter(gz)
	data := []byte("#!/bin/sh\n")
	if err := tw.WriteHeader(&tar.Header{Name: "install.sh", Mode: 0755, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	files, err := extractFeatureArchive(compressed.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "install.sh" || string(files[0].Data) != string(data) {
		t.Fatalf("unexpected archive: %#v", files)
	}
}


func TestExtractFeatureArchiveRejectsTraversal(t *testing.T) {
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	tw := tar.NewWriter(gz)
	data := []byte("bad")
	if err := tw.WriteHeader(&tar.Header{Name: "../escape", Mode: 0644, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := extractFeatureArchive(compressed.Bytes()); err == nil {
		t.Fatal("expected traversal archive to be rejected")
	}
}

func featureIDs(features []ResolvedFeature) []string {
	out := make([]string, 0, len(features))
	for _, feature := range features {
		out = append(out, feature.Manifest.ID)
	}
	return out
}


func TestOfficialOCIDefaults(t *testing.T) {
	if os.Getenv("AKTION_TEST_OCI") != "1" {
		t.Skip("set AKTION_TEST_OCI=1 to resolve public official OCI Features")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	resolver := NewResolver(t.TempDir())
	plan, err := resolver.ResolvePlan(ctx, DefaultBaseImage, DefaultSelections())
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(featureIDs(plan.Features), ",")
	for _, want := range []string{"build", "native", "common-utils", "python", "git-lfs"} {
		if !strings.Contains(","+got+",", ","+want+",") {
			t.Fatalf("official default plan %q missing %q", got, want)
		}
	}
	if !strings.Contains(plan.Dockerfile, "INSTALLTOOLS='false'") {
		t.Fatalf("official Python options were not mapped into Dockerfile:\n%s", plan.Dockerfile)
	}
	runtimePlan, err := resolver.ResolvePlan(ctx, DefaultBaseImage, Selections{
		"ghcr.io/devcontainers/features/go:1": {},
		"ghcr.io/devcontainers/features/rust:1": {},
		"ghcr.io/devcontainers/features/docker-in-docker:4": {
			"version": "latest",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !runtimePlan.Runtime.Init || !runtimePlan.Runtime.Privileged {
		t.Fatalf("official runtime flags not resolved: %#v", runtimePlan.Runtime)
	}
	if !contains(runtimePlan.Runtime.CapAdd, "SYS_PTRACE") || !contains(runtimePlan.Runtime.SecurityOpt, "seccomp=unconfined") {
		t.Fatalf("official Go/Rust runtime metadata missing: %#v", runtimePlan.Runtime)
	}
	if len(runtimePlan.Runtime.Mounts) < 2 || !contains(runtimePlan.Runtime.Entrypoints, "/usr/local/share/docker-init.sh") {
		t.Fatalf("official Docker-in-Docker runtime metadata missing: %#v", runtimePlan.Runtime)
	}

	if os.Getenv("AKTION_TEST_DOCKER") != "1" {
		return
	}
	dir := t.TempDir()
	if err := WriteBuildContext(plan, dir); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, "docker", "build", "-t", "local/aktion-oci-smoke:test", ".")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build official OCI feature runner image: %v", err)
	}
}
