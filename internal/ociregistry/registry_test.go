package ociregistry

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseReferencePreservesTagCase(t *testing.T) {
	ref, err := ParseReference("GHCR.IO/Aktion/Presets/Recommended:ReleaseA")
	if err != nil {
		t.Fatal(err)
	}
	if ref.String() != "ghcr.io/aktion/presets/recommended:ReleaseA" {
		t.Fatalf("changed tag: %s", ref.String())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestFetchBlobRejectsDigestBeforeCacheAccess(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "cache")
	if err := os.MkdirAll(filepath.Join(cache, "blobs", "sha256"), 0755); err != nil {
		t.Fatal(err)
	}
	// Exactly 64 characters, but traverses outside the cache's sha256 directory.
	suffix := "../../../" + strings.Repeat("x", 55)
	victim := filepath.Join(dir, strings.Repeat("x", 55))
	if err := os.WriteFile(victim, []byte("must survive"), 0600); err != nil {
		t.Fatal(err)
	}
	c := NewClient(cache)
	c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Error("invalid digest reached the network")
		return nil, context.Canceled
	})
	for _, bad := range []string{suffix, strings.Repeat("z", 64), strings.Repeat("a", 63)} {
		if _, err := c.FetchBlob(context.Background(), Reference{Registry: "ghcr.io", Repository: "example/test"}, "sha256:"+bad); err == nil {
			t.Fatal("accepted invalid digest")
		}
	}
	if data, err := os.ReadFile(victim); err != nil || string(data) != "must survive" {
		t.Fatalf("file outside cache changed: %q, %v", data, err)
	}
}

func TestParseReference(t *testing.T) {
	ref, err := ParseReference("GHCR.IO/Aktion/Presets/Recommended:1")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Registry != "ghcr.io" || ref.Repository != "aktion/presets/recommended" || ref.Reference != "1" || ref.Digest {
		t.Fatalf("unexpected ref: %#v", ref)
	}
	digest := "sha256:" + strings.Repeat("a", 64)
	ref, err = ParseReference("ghcr.io/aktion/presets/recommended@" + digest)
	if err != nil {
		t.Fatal(err)
	}
	if !ref.Digest || ref.Reference != digest {
		t.Fatalf("unexpected digest ref: %#v", ref)
	}
}
