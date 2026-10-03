package runnerfeatures

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestCRLFBuiltinProducesLinuxBuildContext(t *testing.T) {
	want := []byte("#!/bin/sh\nset -eu\nprintf 'installed\\n'\n")
	for _, newline := range []string{"\n", "\r\n"} {
		source := bytes.ReplaceAll(want, []byte("\n"), []byte(newline))
		files := fstest.MapFS{"features/build/install.sh": {Data: source}}
		script, err := readBuiltinInstall(files, "build")
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		plan := Plan{Dockerfile: "FROM scratch\n", Features: []ResolvedFeature{{Manifest: Manifest{ID: "build"}, Files: []FeatureFile{{Path: "install.sh", Mode: 0755, Data: script}}}}}
		if err := WriteBuildContext(plan, dir); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(dir, "features", "01-build", "install.sh"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("checkout newline=%q generated script=%q, want %q", newline, got, want)
		}
	}
}

func TestAllBuiltinScriptsHaveLinuxShebang(t *testing.T) {
	catalog, err := Catalog()
	if err != nil {
		t.Fatal(err)
	}
	for id, feature := range catalog {
		if bytes.Contains(feature.Install, []byte("\r\n")) || !bytes.HasPrefix(feature.Install, []byte("#!/bin/sh\n")) {
			t.Errorf("builtin %s has a non-Linux script: %q", id, feature.Install)
		}
	}
}
