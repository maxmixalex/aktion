package ociregistry

import (
	"strings"
	"testing"
)

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
