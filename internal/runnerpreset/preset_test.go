package runnerpreset

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zyf722/aktion/internal/ociregistry"
	"github.com/zyf722/aktion/internal/runnerfeatures"
)

func TestExtendsAcceptsStringAndArray(t *testing.T) {
	var one Extends
	if err := one.UnmarshalJSON([]byte(`"defaults"`)); err != nil {
		t.Fatal(err)
	}
	if len(one) != 1 || one[0] != "defaults" {
		t.Fatalf("one=%#v", one)
	}
	var many Extends
	if err := many.UnmarshalJSON([]byte(`["defaults","ghcr.io/example/runner/hpclang:1"]`)); err != nil {
		t.Fatal(err)
	}
	if len(many) != 2 {
		t.Fatalf("many=%#v", many)
	}
}

func TestMergeFeaturesOverridesAndDeletes(t *testing.T) {
	base := runnerfeatures.Selections{
		"builtin:build": {},
		"ghcr.io/devcontainers/features/python:1": {"version": "os-provided", "installTools": false},
	}
	got, err := MergeFeatures(base, runnerfeatures.Selections{
		"builtin:build": nil,
		"builtin:gtk4": {},
		"ghcr.io/devcontainers/features/python:1": {"version": "3.13"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["builtin:build"]; ok {
		t.Fatal("build should be removed")
	}
	if _, ok := got["builtin:gtk4"]; !ok {
		t.Fatal("gtk4 should be added")
	}
	python := got["ghcr.io/devcontainers/features/python:1"]
	if python["version"] != "3.13" || python["installTools"] != false {
		t.Fatalf("python=%#v", python)
	}
}

func TestFetchRunnerPreset(t *testing.T) {
	body := []byte(`{"version":1,"runnerFeatures":{"builtin:build":{},"builtin:gtk4":{}}}`)
	layerDigest := ociregistry.DigestBytes(body)
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/manifests/"):
			manifest := fmt.Sprintf(`{
				"schemaVersion":2,
				"mediaType":"application/vnd.oci.image.manifest.v1+json",
				"artifactType":%q,
				"config":{"mediaType":"application/vnd.oci.empty.v1+json","digest":"sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a","size":2},
				"layers":[{"mediaType":%q,"digest":%q,"size":%d}]
			}`, ArtifactType, LayerMediaType, layerDigest, len(body))
			_, _ = w.Write([]byte(manifest))
		case strings.Contains(r.URL.Path, "/blobs/"):
			_, _ = w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	source := NewSource(t.TempDir())
	source.Registry.HTTP = server.Client()
	registry := strings.TrimPrefix(server.URL, "https://")
	got, err := source.Fetch(context.Background(), registry+"/aktion/recommended:1")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.RunnerFeatures["builtin:gtk4"]; !ok {
		t.Fatalf("features=%#v", got.RunnerFeatures)
	}
	if !strings.Contains(got.Canonical, "@sha256:") {
		t.Fatalf("canonical=%q", got.Canonical)
	}
}
