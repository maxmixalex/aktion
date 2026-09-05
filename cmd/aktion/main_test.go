package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeID(t *testing.T) {
	cases := map[string]string{
		"Repo A":      "repo-a",
		"__Foo.Bar__": "foo-bar",
		"abc_123":     "abc_123",
	}
	for in, want := range cases {
		if got := normalizeID(in); got != want {
			t.Fatalf("normalizeID(%q)=%q want %q", in, got, want)
		}
	}
}

func TestYamlQuote(t *testing.T) {
	if got := yamlQuote("a'b"); got != "'a''b'" {
		t.Fatalf("got %q", got)
	}
}

func TestParseGitHubRepoURL(t *testing.T) {
	o, r, err := parseGitHubRepoURL("https://github.com/zyf722/example")
	if err != nil || o != "zyf722" || r != "example" {
		t.Fatalf("%q %q %v", o, r, err)
	}
	if _, _, err := parseGitHubRepoURL("https://example.com/x/y"); err == nil {
		t.Fatal("expected error")
	}
}

func TestPerRepositoryCapacityOverrides(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "app.pem")
	if err := os.WriteFile(key, []byte("placeholder"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := `{
  "app": {
    "clientId": "Iv123",
    "installationId": 123,
    "privateKeyFile": "./app.pem"
  },
  "defaults": {
    "scaleSetName": "home-docker",
    "minRunners": 0,
    "maxRunners": 1,
    "logLevel": "info"
  },
  "repositories": [
    {
      "id": "repo-a",
      "url": "https://github.com/example/repo-a",
      "enabled": true,
      "minRunners": 1,
      "maxRunners": 3
    },
    {
      "id": "repo-b",
      "url": "https://github.com/example/repo-b",
      "enabled": true
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadConfig(dir, "config.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Repos) != 2 {
		t.Fatalf("got %d repos, want 2", len(loaded.Repos))
	}
	byID := map[string]RepoResolved{}
	for _, r := range loaded.Repos {
		byID[r.ServiceID] = r
	}
	if got := byID["repo-a"]; got.Min != 1 || got.Max != 3 {
		t.Fatalf("repo-a capacity=%d/%d, want 1/3", got.Min, got.Max)
	}
	if got := byID["repo-b"]; got.Min != 0 || got.Max != 1 {
		t.Fatalf("repo-b capacity=%d/%d, want inherited 0/1", got.Min, got.Max)
	}
}

func TestRenderComposeUsesSIGINTForGracefulScaleSetCleanup(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "app.pem")
	if err := os.WriteFile(key, []byte("placeholder"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := `{
  "app": {"clientId":"Iv123","installationId":123,"privateKeyFile":"./app.pem"},
  "defaults": {"scaleSetName":"home-docker","minRunners":0,"maxRunners":1},
  "repositories": [{"id":"repo-a","url":"https://github.com/example/repo-a","enabled":true}]
}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadConfig(dir, "config.json")
	if err != nil {
		t.Fatal(err)
	}
	path, err := renderCompose(loaded)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "stop_signal: SIGINT") {
		t.Fatalf("rendered Compose missing SIGINT stop signal:\n%s", s)
	}
	if !strings.Contains(s, "stop_grace_period: 30s") {
		t.Fatalf("rendered Compose missing stop grace period:\n%s", s)
	}
}

func TestListenerDisplayState(t *testing.T) {
	cases := []struct {
		row     dockerRow
		label   string
		healthy bool
	}{
		{dockerRow{State: "running", Status: "Up 3 minutes"}, "RUNNING", true},
		{dockerRow{State: "restarting", Status: "Restarting (1) 3 seconds ago"}, "RESTARTING", false},
		{dockerRow{State: "exited", Status: "Exited (1)"}, "EXITED", false},
	}
	for _, tc := range cases {
		label, _, healthy := listenerDisplayState(tc.row)
		if label != tc.label || healthy != tc.healthy {
			t.Fatalf("state=%q status=%q => %q/%v, want %q/%v", tc.row.State, tc.row.Status, label, healthy, tc.label, tc.healthy)
		}
	}
}

func TestRenderComposeInjectsProxyIntoListenerForRunnerPassThrough(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "app.pem")
	if err := os.WriteFile(key, []byte("placeholder"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := `{
  "app": {"clientId":"Iv123","installationId":123,"privateKeyFile":"./app.pem"},
  "proxy": {"http":"http://host.docker.internal:3081","https":"http://host.docker.internal:3081","noProxy":"localhost,127.0.0.1"},
  "defaults": {"scaleSetName":"home-docker","minRunners":0,"maxRunners":1},
  "repositories": [{"id":"repo-a","url":"https://github.com/example/repo-a","enabled":true}]
}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadConfig(dir, "config.json")
	if err != nil {
		t.Fatal(err)
	}
	path, err := renderCompose(loaded)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		"AKTION_REPOSITORY: 'example/repo-a'",
		"AKTION_SERVICE_ID: 'repo-a'",
		"http_proxy: 'http://host.docker.internal:3081'",
		"HTTP_PROXY: 'http://host.docker.internal:3081'",
		"https_proxy: 'http://host.docker.internal:3081'",
		"HTTPS_PROXY: 'http://host.docker.internal:3081'",
		"no_proxy: 'localhost,127.0.0.1'",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("rendered Compose missing %q:\n%s", want, s)
		}
	}
}

func TestLogFieldAndWorkflowShort(t *testing.T) {
	line := `time=2026-09-05T10:42:01.123+08:00 level=INFO msg="Job started" scaler.runnerName=runner-abcd1234 scaler.repository=zyf722/hpclang scaler.jobDisplayName="E2E / hp39gs (ubuntu)" scaler.workflowRunId=12345 scaler.jobWorkflowRef=zyf722/hpclang/.github/workflows/e2e.yml@refs/heads/main`
	if got := logField(line, "runnerName"); got != "runner-abcd1234" {
		t.Fatalf("runner=%q", got)
	}
	if got := logField(line, "jobDisplayName"); got != "E2E / hp39gs (ubuntu)" {
		t.Fatalf("job=%q", got)
	}
	if got := workflowShort(logField(line, "jobWorkflowRef")); got != "e2e.yml" {
		t.Fatalf("workflow=%q", got)
	}
}

func TestGitHubLinkTargets(t *testing.T) {
	if got := repoWebURL("zyf722/hpclang"); got != "https://github.com/zyf722/hpclang" {
		t.Fatalf("repo URL=%q", got)
	}
	if got := workflowRunURL("zyf722/hpclang", 12345); got != "https://github.com/zyf722/hpclang/actions/runs/12345" {
		t.Fatalf("run URL=%q", got)
	}
	got := workflowFileURL("zyf722/hpclang", "zyf722/hpclang/.github/workflows/e2e.yml@refs/heads/main")
	if got != "https://github.com/zyf722/hpclang/blob/main/.github/workflows/e2e.yml" {
		t.Fatalf("workflow URL=%q", got)
	}
	if got := workflowFileURL("zyf722/hpclang", "zyf722/hpclang/.github/workflows/e2e.yml@refs/pull/1/merge"); got != "" {
		t.Fatalf("unsupported ref should not fabricate URL: %q", got)
	}
}

func TestRunnerFeaturesInheritanceAndExplicitDisable(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "app.pem")
	if err := os.WriteFile(key, []byte("placeholder"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := "{\n" +
		"  \"app\": {\"clientId\":\"Iv123\",\"installationId\":123,\"privateKeyFile\":\"./app.pem\"},\n" +
		"  \"defaults\": {\n" +
		"    \"scaleSetName\":\"home-docker\",\n" +
		"    \"minRunners\":0,\n" +
		"    \"maxRunners\":1,\n" +
		"    \"runnerFeatures\": {\n" +
		"      \"builtin:build\": {},\n" +
		"      \"ghcr.io/devcontainers/features/python:1\": {\"version\":\"os-provided\",\"installTools\":false}\n" +
		"    }\n" +
		"  },\n" +
		"  \"repositories\": [\n" +
		"    {\"id\":\"repo-a\",\"url\":\"https://github.com/example/repo-a\",\"enabled\":true},\n" +
		"    {\"id\":\"repo-b\",\"url\":\"https://github.com/example/repo-b\",\"enabled\":true,\"runnerFeatures\":{}},\n" +
		"    {\"id\":\"repo-c\",\"url\":\"https://github.com/example/repo-c\",\"enabled\":true,\"runnerFeatures\":{\"builtin:gtk4\":{}}}\n" +
		"  ]\n" +
		"}"
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadConfig(dir, "config.json")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]RepoResolved{}
	for _, r := range loaded.Repos {
		byID[r.ServiceID] = r
	}
	if got := strings.Join(byID["repo-a"].Features, ","); !strings.Contains(got, "builtin:build") || !strings.Contains(got, "ghcr.io/devcontainers/features/python:1") {
		t.Fatalf("repo-a features=%q", got)
	}
	if got := byID["repo-b"]; len(got.Features) != 0 || got.Image != "ghcr.io/actions/actions-runner:latest" {
		t.Fatalf("repo-b should use unmodified base image: features=%v image=%q", got.Features, got.Image)
	}
	gotC := strings.Join(byID["repo-c"].Features, ",")
	if gotC != "builtin:gtk4" {
		t.Fatalf("repo-c configured features=%q", gotC)
	}
}

func TestDefaultRunnerFeaturesAreAppliedWhenOmitted(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "app.pem")
	if err := os.WriteFile(key, []byte("placeholder"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := "{\n" +
		"  \"app\": {\"clientId\":\"Iv123\",\"installationId\":123,\"privateKeyFile\":\"./app.pem\"},\n" +
		"  \"defaults\": {\"scaleSetName\":\"home-docker\",\"minRunners\":0,\"maxRunners\":1},\n" +
		"  \"repositories\": [{\"id\":\"repo-a\",\"url\":\"https://github.com/example/repo-a\",\"enabled\":true}]\n" +
		"}"
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadConfig(dir, "config.json")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(loaded.Repos[0].Features, ",")
	for _, want := range []string{
		"builtin:build",
		"builtin:native",
		"ghcr.io/devcontainers/features/common-utils:2",
		"ghcr.io/devcontainers/features/python:1",
		"ghcr.io/devcontainers/features/git-lfs:1",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("default features %q missing %q", got, want)
		}
	}
	if !strings.HasPrefix(loaded.Repos[0].Image, "local/aktion-runner:") {
		t.Fatalf("expected generated runner image, got %q", loaded.Repos[0].Image)
	}
}

