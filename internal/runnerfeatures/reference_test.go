package runnerfeatures

import (
	"context"
	"strings"
	"testing"
)

func TestResolvePlanPreservesTagCase(t *testing.T) {
	ref := "ghcr.io/example/features/tool:ReleaseA"
	resolver := &Resolver{OCI: fakeFetcher{
		ref: {
			Identity:  "ghcr.io/example/features/tool",
			Canonical: "ghcr.io/example/features/tool@sha256:a",
			Manifest:  Manifest{ID: "tool", Version: "1.0.0", Name: "Tool"},
			Files:     []FeatureFile{{Path: "install.sh", Data: []byte("#!/bin/sh\n")}},
		},
	}}
	plan, err := resolver.ResolvePlan(context.Background(), "base:1", Selections{ref: {}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Features) != 1 || plan.Features[0].Reference != ref {
		t.Fatalf("unexpected resolved features: %#v", plan.Features)
	}
	lowerImage, _, err := ImageName("base:1", Selections{strings.ToLower(ref): {}})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Image == lowerImage {
		t.Fatal("case-sensitive tags share an image cache key")
	}
	if got, err := NormalizeReference("BUILTIN:BUILD"); err != nil || got != "builtin:build" {
		t.Fatalf("builtin normalization changed: %q, %v", got, err)
	}
}
