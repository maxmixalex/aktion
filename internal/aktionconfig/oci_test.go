package aktionconfig

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zyf722/aktion/internal/ociregistry"
)

func TestOCIPresetSource(t *testing.T) {
	yaml := []byte("version: 1\nrunner:\n  features:\n    builtin:build: {}\n")
	layerDigest := ociregistry.DigestBytes(yaml)

	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/manifests/"):
			manifest := fmt.Sprintf(`{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.manifest.v1+json",
  "artifactType": %q,
  "config": {
    "mediaType": "application/vnd.oci.empty.v1+json",
    "digest": "sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
    "size": 2
  },
  "layers": [{
    "mediaType": %q,
    "digest": %q,
    "size": %d
  }]
}`, PresetArtifactType, PresetLayerMediaType, layerDigest, len(yaml))
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			_, _ = w.Write([]byte(manifest))
		case strings.Contains(r.URL.Path, "/blobs/"):
			_, _ = w.Write(yaml)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	registry := strings.TrimPrefix(server.URL, "https://")
	source := NewOCIPresetSource(t.TempDir())
	source.Registry.HTTP = server.Client()

	doc, canonical, err := source.FetchPreset(context.Background(), registry+"/aktion/recommended:1")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Version != 1 {
		t.Fatalf("version=%d", doc.Version)
	}
	if _, ok := doc.Runner.Features["builtin:build"]; !ok {
		t.Fatalf("features=%#v", doc.Runner.Features)
	}
	if !strings.HasPrefix(canonical, registry+"/aktion/recommended@sha256:") {
		t.Fatalf("canonical=%q", canonical)
	}
}
