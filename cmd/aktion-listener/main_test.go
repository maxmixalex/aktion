package main

import (
	"math"
	"testing"
)

func TestRunnerImageFlag(t *testing.T) {
	flag := cmd.Flags().Lookup("runner-image")
	if flag == nil {
		t.Fatal("runner-image flag is not registered")
	}
	if got, want := flag.DefValue, "ghcr.io/actions/actions-runner:latest"; got != want {
		t.Fatalf("runner-image default = %q, want %q", got, want)
	}
}

func TestMaxCapacity(t *testing.T) {
	if got := (Config{MaxRunners: 0}).maxCapacity(); got != math.MaxInt32 {
		t.Fatalf("unlimited max capacity = %d, want %d", got, math.MaxInt32)
	}
	if got := (Config{MaxRunners: 7}).maxCapacity(); got != 7 {
		t.Fatalf("finite max capacity = %d, want 7", got)
	}
}
