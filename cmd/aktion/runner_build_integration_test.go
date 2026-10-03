package main

import (
	"os"
	"testing"
)

// Opt-in: builds the configured runner images using a real Docker daemon.
// It neither starts listeners nor connects to GitHub with the App credentials.
func TestRunnerImageBuildIntegration(t *testing.T) {
	root := os.Getenv("AKTION_TEST_CONFIG_ROOT")
	if root == "" {
		t.Skip("set AKTION_TEST_CONFIG_ROOT to a deployment directory to build its runner images")
	}
	loaded, err := loadConfig(root, "config.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureDocker(); err != nil {
		t.Fatal(err)
	}
	if err := ensureRunnerFeatureImages(loaded); err != nil {
		t.Fatal(err)
	}
}
