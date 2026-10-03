package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigRootIgnoresWorkingDirectory(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		t.Chdir(t.TempDir())
		got, err := resolveConfigRoot("")
		if err != nil {
			t.Fatal(err)
		}
		if got != filepath.Dir(executable) {
			t.Fatalf("root=%q, want executable directory %q", got, filepath.Dir(executable))
		}
	}
}

func TestExplicitConfigRoot(t *testing.T) {
	t.Chdir(t.TempDir())
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	absolute := t.TempDir()
	for _, tc := range []struct{ input, want string }{
		{".", cwd},
		{"config", filepath.Join(cwd, "config")},
		{absolute, absolute},
	} {
		got, err := resolveConfigRoot(tc.input)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Fatalf("root(%q)=%q, want %q", tc.input, got, tc.want)
		}
	}
}
