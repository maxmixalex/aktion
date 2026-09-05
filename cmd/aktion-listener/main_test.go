package main

import "testing"

func TestRunnerImageFlag(t *testing.T) {
	flag := cmd.Flags().Lookup("runner-image")
	if flag == nil {
		t.Fatal("runner-image flag is not registered")
	}
	if got, want := flag.DefValue, "ghcr.io/actions/actions-runner:latest"; got != want {
		t.Fatalf("runner-image default = %q, want %q", got, want)
	}
}
