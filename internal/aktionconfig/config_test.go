package aktionconfig

import (
	"context"
	"strings"
	"testing"

	"github.com/zyf722/aktion/internal/runnerfeatures"
)

type fakePresetSource map[string]struct {
	doc       Document
	canonical string
}

func (f fakePresetSource) FetchPreset(_ context.Context, ref string) (Document, string, error) {
	p, ok := f[ref]
	if !ok {
		return Document{}, "", context.Canceled
	}
	return p.doc, p.canonical, nil
}

func TestParseProjectConfig(t *testing.T) {
	doc, err := Parse([]byte(`version: 1
extends:
  - ghcr.io/aktion/presets/recommended:1
runner:
  features:
    builtin:gtk4: {}
    ghcr.io/devcontainers/features/python:1:
      version: "3.13"
`))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Version != 1 || len(doc.Extends) != 1 {
		t.Fatalf("unexpected document: %#v", doc)
	}
}

func TestFeaturePatchExtendsAndCanRemove(t *testing.T) {
	base := runnerfeatures.Selections{
		"builtin:build": {},
		"ghcr.io/devcontainers/features/python:1": {
			"version":      "os-provided",
			"installTools": false,
		},
	}
	got, err := ApplyFeaturePatch(base, map[string]any{
		"builtin:build": false,
		"builtin:gtk4":  map[string]any{},
		"ghcr.io/devcontainers/features/python:1": map[string]any{
			"version": "3.13",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["builtin:build"]; ok {
		t.Fatal("builtin:build should have been removed")
	}
	if _, ok := got["builtin:gtk4"]; !ok {
		t.Fatal("builtin:gtk4 should have been added")
	}
	python := got["ghcr.io/devcontainers/features/python:1"]
	if python["version"] != "3.13" || python["installTools"] != false {
		t.Fatalf("python options were not shallow-merged: %#v", python)
	}
}

func TestPresetOrderAndProjectOverride(t *testing.T) {
	a := "ghcr.io/aktion/presets/a:1"
	b := "ghcr.io/aktion/presets/b:1"
	source := fakePresetSource{
		a: {
			canonical: "ghcr.io/aktion/presets/a@sha256:" + strings.Repeat("a", 64),
			doc: Document{
				Version: 1,
				Runner: Runner{Features: map[string]any{
					"ghcr.io/devcontainers/features/python:1": map[string]any{
						"version":      "3.12",
						"installTools": false,
					},
				}},
			},
		},
		b: {
			canonical: "ghcr.io/aktion/presets/b@sha256:" + strings.Repeat("b", 64),
			doc: Document{
				Version: 1,
				Runner: Runner{Features: map[string]any{
					"ghcr.io/devcontainers/features/python:1": map[string]any{
						"version": "3.13",
					},
					"builtin:native": map[string]any{},
				}},
			},
		},
	}
	resolved, err := (&Resolver{Presets: source}).Resolve(context.Background(), Document{
		Version: 1,
		Extends: []string{a, b},
		Runner: Runner{Features: map[string]any{
			"ghcr.io/devcontainers/features/python:1": map[string]any{
				"installTools": true,
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	python := resolved.Features["ghcr.io/devcontainers/features/python:1"]
	if python["version"] != "3.13" || python["installTools"] != true {
		t.Fatalf("unexpected merged python options: %#v", python)
	}
	if _, ok := resolved.Features["builtin:native"]; !ok {
		t.Fatal("expected native feature from second preset")
	}
	if len(resolved.Presets) != 2 {
		t.Fatalf("presets=%#v", resolved.Presets)
	}
}

func TestPresetCycleRejected(t *testing.T) {
	a := "ghcr.io/aktion/presets/a:1"
	b := "ghcr.io/aktion/presets/b:1"
	source := fakePresetSource{
		a: {
			canonical: "a@sha256:" + strings.Repeat("a", 64),
			doc:       Document{Version: 1, Extends: []string{b}},
		},
		b: {
			canonical: "b@sha256:" + strings.Repeat("b", 64),
			doc:       Document{Version: 1, Extends: []string{a}},
		},
	}
	_, err := (&Resolver{Presets: source}).Resolve(context.Background(), Document{
		Version: 1,
		Extends: []string{a},
	})
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got %v", err)
	}
}
