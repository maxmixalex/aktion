package runnerfeatures

import (
	"strings"
	"testing"
)

func ids(features []Feature) []string {
	out := make([]string, 0, len(features))
	for _, f := range features {
		out = append(out, f.Manifest.ID)
	}
	return out
}

func TestResolveAddsDependenciesAndOrdersThem(t *testing.T) {
	features, err := Resolve([]string{"gtk4", "lua54"})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(ids(features), ",")
	for _, required := range []string{"build", "glib", "gtk4", "lua54"} {
		if !strings.Contains(","+got+",", ","+required+",") {
			t.Fatalf("resolved features %q missing %q", got, required)
		}
	}
	index := map[string]int{}
	for i, id := range ids(features) {
		index[id] = i
	}
	if index["build"] > index["glib"] || index["glib"] > index["gtk4"] {
		t.Fatalf("dependency order is wrong: %v", ids(features))
	}
}

func TestResolveRejectsUnknownFeature(t *testing.T) {
	if _, err := Resolve([]string{"does-not-exist"}); err == nil {
		t.Fatal("expected unknown feature error")
	}
}

func TestBuildPlanIsStableAndLayered(t *testing.T) {
	p1, err := BuildPlan("ghcr.io/actions/actions-runner:latest", []string{"build", "python"})
	if err != nil {
		t.Fatal(err)
	}
	p2, err := BuildPlan("ghcr.io/actions/actions-runner:latest", []string{"build", "python"})
	if err != nil {
		t.Fatal(err)
	}
	if p1.Image != p2.Image {
		t.Fatalf("image tag not stable: %q != %q", p1.Image, p2.Image)
	}
	if !strings.HasPrefix(p1.Image, "local/aktion-runner:") {
		t.Fatalf("unexpected image tag %q", p1.Image)
	}
	if strings.Count(p1.Dockerfile, " AS aktion-feature-") != 2 {
		t.Fatalf("expected one build stage per feature:\n%s", p1.Dockerfile)
	}
	if !strings.Contains(p1.Dockerfile, "FROM ghcr.io/actions/actions-runner:latest AS aktion-base") {
		t.Fatalf("missing base stage:\n%s", p1.Dockerfile)
	}
	if !strings.HasSuffix(p1.Dockerfile, "USER runner\n") {
		t.Fatalf("final image must return to runner user:\n%s", p1.Dockerfile)
	}
}

func TestEmptyFeaturesUsesBaseImageDirectly(t *testing.T) {
	p, err := BuildPlan("example/base:1", []string{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Image != "example/base:1" || p.Dockerfile != "" || len(p.Features) != 0 {
		t.Fatalf("unexpected empty feature plan: %#v", p)
	}
}
