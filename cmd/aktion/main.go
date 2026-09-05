package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/zyf722/aktion/internal/runnerfeatures"
	"github.com/zyf722/aktion/internal/runnerpreset"
)

const githubAPIVersion = "2026-03-10"

var version = "dev"

type Config struct {
	App          AppConfig    `json:"app"`
	Proxy        ProxyConfig  `json:"proxy,omitempty"`
	Defaults     Defaults     `json:"defaults"`
	Repositories []Repository `json:"repositories"`
}

type ProxyConfig struct {
	HTTP    string `json:"http,omitempty"`
	HTTPS   string `json:"https,omitempty"`
	NoProxy string `json:"noProxy,omitempty"`
}

type AppConfig struct {
	ClientID       string `json:"clientId"`
	InstallationID int64  `json:"installationId"`
	PrivateKeyFile string `json:"privateKeyFile"`
}

type Defaults struct {
	Extends        []string      `json:"extends,omitempty"`
	ScaleSetName   string                    `json:"scaleSetName"`
	MinRunners     *int                      `json:"minRunners"`
	MaxRunners     *int                      `json:"maxRunners"`
	LogLevel       string                    `json:"logLevel"`
	Labels         string                    `json:"labels"`
	RunnerImage    string                    `json:"runnerImage"`
	RunnerFeatures runnerfeatures.Selections `json:"runnerFeatures,omitempty"`
}

type Repository struct {
	ID             string                    `json:"id"`
	URL            string                    `json:"url"`
	Enabled        bool                      `json:"enabled"`
	Extends        []string      `json:"extends,omitempty"`
	InstallationID *int64                    `json:"installationId,omitempty"`
	ScaleSetName   string                    `json:"scaleSetName,omitempty"`
	MinRunners     *int                      `json:"minRunners,omitempty"`
	MaxRunners     *int                      `json:"maxRunners,omitempty"`
	LogLevel       string                    `json:"logLevel,omitempty"`
	Labels         string                    `json:"labels,omitempty"`
	RunnerImage    string                    `json:"runnerImage,omitempty"`
	RunnerFeatures runnerfeatures.Selections `json:"runnerFeatures,omitempty"`
}

type RepoResolved struct {
	Repository
	ServiceID          string
	Installation       int64
	ScaleSet           string
	Min                int
	Max                int
	Log                string
	LabelsResolved     string
	Image              string
	BaseImage          string
	Features           []string
	FeatureSelections  runnerfeatures.Selections
}

type Loaded struct {
	Root       string
	KeyPath    string
	Config     Config
	Repos      []RepoResolved
}

type dockerRow struct {
	Image      string `json:"Image"`
	Names      string `json:"Names"`
	Status     string `json:"Status"`
	State      string `json:"State"`
	Labels     string `json:"Labels"`
}

type activeJob struct {
	Repository  string
	JobID       string
	JobName     string
	WorkflowRef string
	WorkflowRun int64
	StartedAt   time.Time
}

type repoDemand struct {
	Assigned int
}

type dockerStat struct {
	Name     string `json:"Name"`
	CPUPerc  string `json:"CPUPerc"`
	MemUsage string `json:"MemUsage"`
	MemPerc  string `json:"MemPerc"`
	NetIO    string `json:"NetIO"`
}

type githubAccessToken struct {
	Token string `json:"token"`
}

type runnerRegistrationToken struct {
	Token string `json:"token"`
}

type actionsAdminConnection struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

type remoteScaleSet struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	RunnerGroupName string `json:"runnerGroupName"`
}

type remoteScaleSetList struct {
	Count int              `json:"count"`
	Value []remoteScaleSet `json:"value"`
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	rootDefault, _ := os.Getwd()
	fs := flag.NewFlagSet("aktion", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", rootDefault, "configuration directory")
	configName := fs.String("config", "config.json", "configuration file name")
	colorMode := fs.String("color", "auto", "color output: auto, always, never")
	linkMode := fs.String("links", "auto", "terminal hyperlinks: auto, always, never")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		printUsage()
		return 2
	}
	var err error
	ui, err = newUI(*colorMode, *linkMode)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	rest := fs.Args()
	cmd := "status"
	if len(rest) > 0 {
		cmd = strings.ToLower(rest[0])
		rest = rest[1:]
	}
	absRoot, err := filepath.Abs(*root)
	if err != nil {
		return fail(err)
	}

	switch cmd {
	case "help", "-h", "--help":
		printUsage()
		return 0
	case "version":
		fmt.Printf("aktion %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		return 0
	case "features":
		return cmdFeatures()
	}

	loaded, err := loadConfig(absRoot, *configName)
	if err != nil {
		return fail(err)
	}


	switch cmd {
	case "render":
		p, err := renderCompose(loaded)
		if err != nil {
			return fail(err)
		}
		fmt.Println("Generated:", p)
		return 0
	case "config":
		if len(rest) > 0 && strings.EqualFold(rest[0], "resolve") {
			if len(rest) != 2 {
				return fail(errors.New("usage: aktion config resolve <repo-id>"))
			}
			return cmdConfigResolve(loaded, rest[1])
		}
		if len(rest) > 0 {
			return fail(fmt.Errorf("unknown config subcommand %q", rest[0]))
		}
		p, err := renderCompose(loaded)
		if err != nil {
			return fail(err)
		}
		if err := ensureDocker(); err != nil {
			return fail(err)
		}
		return runStreaming(loaded.Root, "docker", "compose", "-f", p, "config")
	case "up":
		p, err := renderCompose(loaded)
		if err != nil {
			return fail(err)
		}
		if err := ensureDocker(); err != nil {
			return fail(err)
		}
		if err := ensureRunnerFeatureImages(loaded); err != nil {
			return fail(err)
		}
		code := runStreaming(loaded.Root, "docker", "compose", "-f", p, "up", "-d", "--build")
		if code == 0 {
			fmt.Println()
			fmt.Println(ui.renderStyle("good", "Scale-set listeners are up."))
			fmt.Println(ui.renderStyle("muted", "Use: aktion status"))
			fmt.Println(ui.renderStyle("muted", "Use: aktion logs"))
		}
		return code
	case "down":
		p, err := ensureRendered(loaded)
		if err != nil {
			return fail(err)
		}
		if err := ensureDocker(); err != nil {
			return fail(err)
		}
		return runStreaming(loaded.Root, "docker", "compose", "-f", p, "down")
	case "restart":
		p, err := renderCompose(loaded)
		if err != nil {
			return fail(err)
		}
		if err := ensureDocker(); err != nil {
			return fail(err)
		}
		if code := runStreaming(loaded.Root, "docker", "compose", "-f", p, "down"); code != 0 {
			return code
		}
		if err := ensureRunnerFeatureImages(loaded); err != nil {
			return fail(err)
		}
		return runStreaming(loaded.Root, "docker", "compose", "-f", p, "up", "-d", "--build")
	case "cleanup":
		return cmdCleanup(loaded, rest)
	case "scalesets":
		return cmdScaleSets(loaded)
	case "logs":
		p, err := ensureRendered(loaded)
		if err != nil {
			return fail(err)
		}
		if err := ensureDocker(); err != nil {
			return fail(err)
		}
		args := []string{"compose", "-f", p, "logs", "-f", "--tail", "200"}
		if len(rest) > 0 {
			sid, err := findRepoService(loaded, rest[0])
			if err != nil {
				return fail(err)
			}
			args = append(args, "listener-"+sid)
		}
		return runStreaming(loaded.Root, "docker", args...)
	case "repos":
		printRepos(loaded)
		return 0
	case "runners":
		if err := ensureDocker(); err != nil {
			return fail(err)
		}
		rows, err := dockerPS()
		if err != nil {
			return fail(err)
		}
		images := configuredRunnerImages(loaded)
		jobs := filterJobsToRunningContainers(collectActiveJobs(loaded), rows, images)
		printRunners(loaded, rows, jobs)
		return 0
	case "stats":
		if err := ensureDocker(); err != nil {
			return fail(err)
		}
		return cmdStats(loaded)
	case "status":
		if err := ensureDocker(); err != nil {
			return fail(err)
		}
		return cmdStatus(loaded)
	case "doctor":
		return cmdDoctor(loaded)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		return 2
	}
}

func printUsage() {
	fmt.Println(`aktion - local manager for GitHub Actions Docker runner scale sets

Usage:
  aktion [--root DIR] [--color auto|always|never] [--links auto|always|never] <command> [args]

Commands:
  features          List Aktion builtins and default OCI Features
  config            Render and validate Compose configuration
  config resolve ID Resolve the effective runner config for one repository
  render            Generate compose.generated.yaml
  up                Build/start all enabled listeners
  down              Stop/remove listener stack
  restart           Recreate listener stack
  cleanup           Remove stale remote scale sets, then restart listeners
  scalesets         Show remote GitHub runner scale sets for configured repos
  status            Show repository/listener and runner state
  repos             Show configured repositories
  runners           Show active ephemeral runner containers
  stats             Show live Docker CPU/memory snapshot
  logs [repo-id]    Follow all listener logs or one repository listener
  doctor            Check config, Docker, GitHub App and installations
  version           Show CLI version

Color:
  --color auto     Color only when stdout/stderr is a terminal (default)
  --color always   Force ANSI color
  --color never    Disable ANSI color
  NO_COLOR=1       Also disables color in auto mode

Links:
  --links auto     OSC 8 hyperlinks only when stdout is a terminal (default)
  --links always   Force OSC 8 hyperlinks
  --links never    Disable terminal hyperlinks
  NO_HYPERLINK=1   Disable hyperlinks in auto mode

Examples:
  aktion doctor
  aktion config resolve repo-a
  aktion up
  aktion status
  aktion scalesets
  aktion cleanup
  aktion logs repo-a`)
}


func intOr(p *int, d int) int {
	if p != nil {
		return *p
	}
	return d
}
func strOr(v, d string) string {
	if v != "" {
		return v
	}
	return d
}

func loadConfig(root, configName string) (Loaded, error) {
	p := filepath.Join(root, configName)
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Loaded{}, fmt.Errorf("missing configuration file: %s", p)
		}
		return Loaded{}, err
	}
	var cfg Config
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Loaded{}, fmt.Errorf("invalid config JSON: %w", err)
	}

	if cfg.App.ClientID == "" {
		return Loaded{}, errors.New("app.clientId is required")
	}
	if cfg.App.InstallationID == 0 {
		return Loaded{}, errors.New("app.installationId is required")
	}
	if cfg.App.PrivateKeyFile == "" {
		return Loaded{}, errors.New("app.privateKeyFile is required")
	}
	if err := validateProxyConfig(cfg.Proxy); err != nil {
		return Loaded{}, err
	}
	keyPath := cfg.App.PrivateKeyFile
	if !filepath.IsAbs(keyPath) {
		keyPath = filepath.Join(root, filepath.FromSlash(keyPath))
	}
	keyPath, _ = filepath.Abs(keyPath)
	if st, err := os.Stat(keyPath); err != nil || st.IsDir() {
		return Loaded{}, fmt.Errorf("GitHub App private key not found: %s", keyPath)
	}

	defName := strOr(cfg.Defaults.ScaleSetName, "home-docker")
	defMin := intOr(cfg.Defaults.MinRunners, 0)
	defMax := intOr(cfg.Defaults.MaxRunners, 1)
	defLog := strOr(cfg.Defaults.LogLevel, "info")
	if err := validateCapacity("defaults", defMin, defMax); err != nil {
		return Loaded{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	presetSource := runnerpreset.NewSource(runnerfeatures.DefaultCacheDir())
	presetCache := map[string]runnerpreset.Resolved{}
	fetchPreset := func(ref string) (runnerpreset.Resolved, error) {
		if cached, ok := presetCache[ref]; ok {
			return cached, nil
		}
		preset, err := presetSource.Fetch(ctx, ref)
		if err != nil {
			return runnerpreset.Resolved{}, err
		}
		presetCache[ref] = preset
		return preset, nil
	}

	defImage := ""
	defFeatures := runnerfeatures.DefaultSelections()
	if cfg.Defaults.Extends != nil {
		defFeatures = runnerfeatures.Selections{}
		for _, ref := range cfg.Defaults.Extends {
			if ref == "defaults" {
				return Loaded{}, errors.New("defaults.extends cannot reference defaults")
			}
			preset, err := fetchPreset(ref)
			if err != nil {
				return Loaded{}, fmt.Errorf("defaults.extends %q: %w", ref, err)
			}
			defFeatures, err = runnerpreset.MergeFeatures(defFeatures, preset.RunnerFeatures)
			if err != nil {
				return Loaded{}, fmt.Errorf("defaults.extends %q: %w", ref, err)
			}
			if preset.RunnerImage != "" {
				defImage = preset.RunnerImage
			}
		}
	}
	if cfg.Defaults.RunnerFeatures != nil {
		defFeatures, err = runnerpreset.MergeFeatures(defFeatures, cfg.Defaults.RunnerFeatures)
		if err != nil {
			return Loaded{}, fmt.Errorf("defaults.runnerFeatures: %w", err)
		}
	}
	if cfg.Defaults.RunnerImage != "" {
		defImage = cfg.Defaults.RunnerImage
	}

	seen := map[string]bool{}
	var repos []RepoResolved
	for _, r := range cfg.Repositories {
		if !r.Enabled {
			continue
		}
		if r.ID == "" {
			return Loaded{}, errors.New("every enabled repository needs an id")
		}
		if r.URL == "" {
			return Loaded{}, fmt.Errorf("repository %q needs a url", r.ID)
		}
		if _, _, err := parseGitHubRepoURL(r.URL); err != nil {
			return Loaded{}, fmt.Errorf("repository %q: %w", r.ID, err)
		}
		sid := normalizeID(r.ID)
		if sid == "" {
			return Loaded{}, fmt.Errorf("repository id %q cannot be converted to a valid service name", r.ID)
		}
		if seen[sid] {
			return Loaded{}, fmt.Errorf("duplicate repository/service id after normalization: %s", sid)
		}
		seen[sid] = true
		min := intOr(r.MinRunners, defMin)
		max := intOr(r.MaxRunners, defMax)
		if err := validateCapacity(sid, min, max); err != nil {
			return Loaded{}, err
		}
		inst := cfg.App.InstallationID
		if r.InstallationID != nil {
			inst = *r.InstallationID
		}
		features := runnerfeatures.CloneSelections(defFeatures)
		baseImage := defImage
		if r.Extends != nil {
			features = runnerfeatures.Selections{}
			baseImage = ""
			for _, ref := range r.Extends {
				if ref == "defaults" {
					features, err = runnerpreset.MergeFeatures(features, defFeatures)
					if err != nil {
						return Loaded{}, fmt.Errorf("repository %q extends defaults: %w", r.ID, err)
					}
					if defImage != "" {
						baseImage = defImage
					}
					continue
				}
				preset, err := fetchPreset(ref)
				if err != nil {
					return Loaded{}, fmt.Errorf("repository %q extends %q: %w", r.ID, ref, err)
				}
				features, err = runnerpreset.MergeFeatures(features, preset.RunnerFeatures)
				if err != nil {
					return Loaded{}, fmt.Errorf("repository %q extends %q: %w", r.ID, ref, err)
				}
				if preset.RunnerImage != "" {
					baseImage = preset.RunnerImage
				}
			}
		}
		if r.RunnerFeatures != nil {
			features, err = runnerpreset.MergeFeatures(features, r.RunnerFeatures)
			if err != nil {
				return Loaded{}, fmt.Errorf("repository %q runnerFeatures: %w", r.ID, err)
			}
		}
		if r.RunnerImage != "" {
			baseImage = r.RunnerImage
		}
		imageName, normalizedBaseImage, err := runnerfeatures.ImageName(baseImage, features)
		if err != nil {
			return Loaded{}, fmt.Errorf("repository %q runnerFeatures: %w", r.ID, err)
		}
		baseImage = normalizedBaseImage
		featureIDs := runnerfeatures.SelectionKeys(features)
		repos = append(repos, RepoResolved{
			Repository:     r,
			ServiceID:      sid,
			Installation:   inst,
			ScaleSet:       strOr(r.ScaleSetName, defName),
			Min:            min,
			Max:            max,
			Log:            strOr(r.LogLevel, defLog),
			LabelsResolved: strOr(r.Labels, cfg.Defaults.Labels),
			Image:             imageName,
			BaseImage:         baseImage,
			Features:          featureIDs,
			FeatureSelections: features,
		})
	}
	if len(repos) == 0 {
		return Loaded{}, errors.New("no enabled repositories in config.json")
	}
	sort.Slice(repos, func(i, j int) bool { return repos[i].ServiceID < repos[j].ServiceID })
	return Loaded{Root: root, KeyPath: keyPath, Config: cfg, Repos: repos}, nil
}




func cmdConfigResolve(l Loaded, id string) int {
	for _, r := range l.Repos {
		if r.ID != id && r.ServiceID != normalizeID(id) {
			continue
		}
		owner, repo, _ := parseGitHubRepoURL(r.URL)
		payload := struct {
			Repository string                    `json:"repository"`
			Extends    []string                  `json:"extends,omitempty"`
			MinRunners int                       `json:"minRunners"`
			MaxRunners int                       `json:"maxRunners"`
			BaseImage  string                    `json:"baseImage"`
			Image      string                    `json:"image"`
			Features   runnerfeatures.Selections `json:"features"`
		}{
			Repository: owner + "/" + repo,
			Extends:    r.Extends,
			MinRunners: r.Min,
			MaxRunners: r.Max,
			BaseImage:  r.BaseImage,
			Image:      r.Image,
			Features:   r.FeatureSelections,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		if err := enc.Encode(payload); err != nil {
			return fail(err)
		}
		return 0
	}
	return fail(fmt.Errorf("unknown enabled repository id %q", id))
}

func cmdFeatures() int {
	catalog, err := runnerfeatures.Catalog()
	if err != nil {
		return fail(err)
	}
	defaults := runnerfeatures.DefaultSelections()
	rows := make([][]tableCell, 0, len(catalog)+len(defaults))
	for _, ref := range runnerfeatures.SelectionKeys(defaults) {
		if strings.HasPrefix(ref, "builtin:") {
			continue
		}
		rows = append(rows, []tableCell{
			accentCell(ref),
			goodCell("DEFAULT"),
			cell("OCI Dev Container Feature"),
			mutedCell("publisher-managed"),
		})
	}
	ids := make([]string, 0, len(catalog))
	for id := range catalog {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		ref := "builtin:" + id
		def := ""
		if _, ok := defaults[ref]; ok {
			def = "DEFAULT"
		}
		f := catalog[id]
		deps := make([]string, 0, len(f.Manifest.DependsOn))
		for dep := range f.Manifest.DependsOn {
			deps = append(deps, dep)
		}
		sort.Strings(deps)
		rows = append(rows, []tableCell{
			accentCell(ref),
			mutedCell(def),
			cell(f.Manifest.Description),
			mutedCell(strings.Join(deps, ", ")),
		})
	}
	ui.section("Runner features")
	fmt.Println(ui.table([]string{"FEATURE", "DEFAULT", "DESCRIPTION", "DEPENDS ON"}, rows))
	fmt.Println()
	fmt.Println(ui.renderStyle("muted", "Any public OCI Dev Container Feature URI is supported, e.g. ghcr.io/devcontainers/features/node:1."))
	fmt.Println(ui.renderStyle("muted", "Omit defaults.runnerFeatures to use Aktion defaults; use extends: [] for an unmodified base runner image."))
	return 0
}

func ensureRunnerFeatureImages(l Loaded) error {
	type imageBuild struct {
		Base       string
		Selections runnerfeatures.Selections
		Image      string
	}
	unique := map[string]imageBuild{}
	for _, r := range l.Repos {
		if len(r.FeatureSelections) == 0 || r.Image == r.BaseImage {
			continue
		}
		unique[r.Image] = imageBuild{Base: r.BaseImage, Selections: r.FeatureSelections, Image: r.Image}
	}
	if len(unique) == 0 {
		return nil
	}
	keys := make([]string, 0, len(unique))
	for image := range unique {
		keys = append(keys, image)
	}
	sort.Strings(keys)
	resolver := runnerfeatures.NewResolver(runnerfeatures.DefaultCacheDir())
	ctx := context.Background()
	for _, image := range keys {
		build := unique[image]
		fmt.Printf("Resolving runner features for %s\n", image)
		plan, err := resolver.ResolvePlan(ctx, build.Base, build.Selections)
		if err != nil {
			return err
		}
		if plan.Image != build.Image {
			return fmt.Errorf("runner image plan changed unexpectedly: %s != %s", plan.Image, build.Image)
		}
		for _, warning := range plan.Warnings {
			fmt.Println(ui.renderStyle("warn", "warning: "+warning))
		}
		dir, err := os.MkdirTemp("", "aktion-runner-image-")
		if err != nil {
			return err
		}
		if err := runnerfeatures.WriteBuildContext(plan, dir); err != nil {
			os.RemoveAll(dir)
			return err
		}
		refs := make([]string, 0, len(plan.Features))
		for _, feature := range plan.Features {
			refs = append(refs, feature.Reference)
		}
		fmt.Printf("Building runner image %s (%s)\n", image, strings.Join(refs, ", "))
		code := runStreaming(dir, "docker", "build", "-t", image, ".")
		os.RemoveAll(dir)
		if code != 0 {
			return fmt.Errorf("failed to build runner image %s", image)
		}
	}
	return nil
}

func validateProxyConfig(p ProxyConfig) error {
	for name, value := range map[string]string{"proxy.http": p.HTTP, "proxy.https": p.HTTPS} {
		if strings.TrimSpace(value) == "" {
			continue
		}
		u, err := url.Parse(value)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("%s must be a proxy URL such as http://host.docker.internal:3081", name)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("%s must use http:// or https://", name)
		}
		host := strings.ToLower(u.Hostname())
		if host == "localhost" || host == "127.0.0.1" || host == "::1" {
			return fmt.Errorf("%s points to %s; runner containers cannot reach the Docker host through loopback; use host.docker.internal instead", name, u.Hostname())
		}
	}
	return nil
}

func proxyEnabled(p ProxyConfig) bool {
	return strings.TrimSpace(p.HTTP) != "" || strings.TrimSpace(p.HTTPS) != ""
}

func redactProxyURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.User != nil {
		u.User = url.UserPassword("***", "***")
	}
	return u.String()
}

func validateCapacity(name string, min, max int) error {
	if min < 0 || max < 0 {
		return fmt.Errorf("%s: minRunners/maxRunners cannot be negative", name)
	}
	if max > 0 && min > max {
		return fmt.Errorf("%s: minRunners cannot exceed maxRunners", name)
	}
	return nil
}

func formatMaxRunners(max int) string {
	if max == 0 {
		return "∞"
	}
	return strconv.Itoa(max)
}

var nonService = regexp.MustCompile(`[^a-z0-9_-]+`)
var trimService = regexp.MustCompile(`^[-_]+|[-_]+$`)

func normalizeID(s string) string {
	s = strings.ToLower(s)
	s = nonService.ReplaceAllString(s, "-")
	return trimService.ReplaceAllString(s, "")
}

func yamlQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func renderCompose(l Loaded) (string, error) {
	var b strings.Builder
	b.WriteString("# GENERATED by Aktion. Do not edit directly.\n")
	b.WriteString("name: aktion\n\nservices:\n")
	imageTag := "local/aktion-listener:" + strings.TrimPrefix(version, "v")
	if imageTag == "local/aktion-listener:" || imageTag == "local/aktion-listener:dev" {
		imageTag = "local/aktion-listener:dev"
	}
	for _, r := range l.Repos {
		fmt.Fprintf(&b, "  listener-%s:\n", r.ServiceID)
		b.WriteString("    build:\n      context: .\n")
		b.WriteString("      dockerfile: Dockerfile\n")
		fmt.Fprintf(&b, "    image: %s\n", yamlQuote(imageTag))
		fmt.Fprintf(&b, "    container_name: %s\n", yamlQuote("aktion-"+r.ServiceID))
		b.WriteString("    restart: unless-stopped\n")
		b.WriteString("    stop_signal: SIGINT\n")
		b.WriteString("    stop_grace_period: 30s\n")
		b.WriteString("    extra_hosts:\n      - host.docker.internal:host-gateway\n")
		b.WriteString("    environment:\n")
		owner, repo, _ := parseGitHubRepoURL(r.URL)
		fmt.Fprintf(&b, "      AKTION_REPOSITORY: %s\n", yamlQuote(owner+"/"+repo))
		fmt.Fprintf(&b, "      AKTION_SERVICE_ID: %s\n", yamlQuote(r.ServiceID))
		fmt.Fprintf(&b, "      GITHUB_CONFIG_URL: %s\n", yamlQuote(strings.TrimRight(r.URL, "/")))
		fmt.Fprintf(&b, "      SCALE_SET_NAME: %s\n", yamlQuote(r.ScaleSet))
		fmt.Fprintf(&b, "      GITHUB_APP_CLIENT_ID: %s\n", yamlQuote(l.Config.App.ClientID))
		fmt.Fprintf(&b, "      GITHUB_APP_INSTALLATION_ID: %s\n", yamlQuote(strconv.FormatInt(r.Installation, 10)))
		fmt.Fprintf(&b, "      MIN_RUNNERS: %s\n", yamlQuote(strconv.Itoa(r.Min)))
		fmt.Fprintf(&b, "      MAX_RUNNERS: %s\n", yamlQuote(strconv.Itoa(r.Max)))
		fmt.Fprintf(&b, "      LOG_LEVEL: %s\n", yamlQuote(r.Log))
		fmt.Fprintf(&b, "      LABELS: %s\n", yamlQuote(r.LabelsResolved))
		fmt.Fprintf(&b, "      RUNNER_IMAGE: %s\n", yamlQuote(r.Image))
		if l.Config.Proxy.HTTP != "" {
			fmt.Fprintf(&b, "      http_proxy: %s\n", yamlQuote(l.Config.Proxy.HTTP))
			fmt.Fprintf(&b, "      HTTP_PROXY: %s\n", yamlQuote(l.Config.Proxy.HTTP))
		}
		if l.Config.Proxy.HTTPS != "" {
			fmt.Fprintf(&b, "      https_proxy: %s\n", yamlQuote(l.Config.Proxy.HTTPS))
			fmt.Fprintf(&b, "      HTTPS_PROXY: %s\n", yamlQuote(l.Config.Proxy.HTTPS))
		}
		if l.Config.Proxy.NoProxy != "" {
			fmt.Fprintf(&b, "      no_proxy: %s\n", yamlQuote(l.Config.Proxy.NoProxy))
			fmt.Fprintf(&b, "      NO_PROXY: %s\n", yamlQuote(l.Config.Proxy.NoProxy))
		}
		b.WriteString("    volumes:\n      - /var/run/docker.sock:/var/run/docker.sock\n")
		b.WriteString("    secrets:\n      - github_app_private_key\n\n")
	}
	b.WriteString("secrets:\n  github_app_private_key:\n")
	// Compose should receive a path relative to the project root when possible.
	rel, err := filepath.Rel(l.Root, l.KeyPath)
	if err != nil {
		rel = l.KeyPath
	}
	rel = filepath.ToSlash(rel)
	fmt.Fprintf(&b, "    file: %s\n", yamlQuote(rel))

	p := filepath.Join(l.Root, "compose.generated.yaml")
	if err := os.WriteFile(p, []byte(b.String()), 0644); err != nil {
		return "", err
	}
	return p, nil
}

func ensureRendered(l Loaded) (string, error) {
	p := filepath.Join(l.Root, "compose.generated.yaml")
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	return renderCompose(l)
}

func ensureDocker() error {
	c := exec.Command("docker", "version")
	c.Stdout = io.Discard
	c.Stderr = io.Discard
	if err := c.Run(); err != nil {
		return errors.New("Docker is unavailable; ensure the Docker Engine/Desktop is running and Linux containers are available")
	}
	return nil
}

func runStreaming(dir, name string, args ...string) int {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

func commandOutput(dir, name string, args ...string) ([]byte, error) {
	c := exec.Command(name, args...)
	c.Dir = dir
	var stderr bytes.Buffer
	c.Stderr = &stderr
	out, err := c.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("%s: %s", err, msg)
		}
		return nil, err
	}
	return out, nil
}

func dockerPS() ([]dockerRow, error) {
	return dockerPSMode(false)
}

func dockerPSAll() ([]dockerRow, error) {
	return dockerPSMode(true)
}

func dockerPSMode(all bool) ([]dockerRow, error) {
	args := []string{"ps"}
	if all {
		args = append(args, "-a")
	}
	args = append(args, "--format", "{{json .}}")
	out, err := commandOutput("", "docker", args...)
	if err != nil {
		return nil, err
	}
	var rows []dockerRow
	for _, line := range bytes.Split(bytes.TrimSpace(out), []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var r dockerRow
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, fmt.Errorf("parse docker ps: %w", err)
		}
		rows = append(rows, r)
	}
	return rows, nil
}

func listenerDisplayState(d dockerRow) (label, tone string, healthy bool) {
	state := strings.ToLower(strings.TrimSpace(d.State))
	status := strings.ToLower(strings.TrimSpace(d.Status))
	switch {
	case state == "running" && !strings.Contains(status, "restarting"):
		return "RUNNING", "good", true
	case state == "restarting" || strings.Contains(status, "restarting"):
		return "RESTARTING", "bad", false
	case state == "exited" || strings.Contains(status, "exited"):
		return "EXITED", "bad", false
	case state == "paused":
		return "PAUSED", "warn", false
	case state == "created":
		return "CREATED", "warn", false
	default:
		if d.State != "" {
			return strings.ToUpper(d.State), "warn", false
		}
		return "UNKNOWN", "warn", false
	}
}

func combinedOutput(dir, name string, args ...string) ([]byte, error) {
	c := exec.Command(name, args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return out, fmt.Errorf("%s: %s", err, msg)
		}
		return out, err
	}
	return out, nil
}

func lastContainerError(name string) string {
	out, err := combinedOutput("", "docker", "logs", "--tail", "80", name)
	if err != nil && len(out) == 0 {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		low := strings.ToLower(line)
		if strings.Contains(line, "RunnerScaleSetExistsException") ||
			strings.Contains(low, "failed to create runner scale set") ||
			strings.Contains(low, "listener run failed") ||
			strings.HasPrefix(low, "error:") {
			if len(line) > 220 {
				line = line[:217] + "..."
			}
			return line
		}
	}
	if len(lines) > 0 {
		line := strings.TrimSpace(lines[len(lines)-1])
		if len(line) > 220 {
			line = line[:217] + "..."
		}
		return line
	}
	return ""
}

func listenerMap(l Loaded) map[string]RepoResolved {
	m := map[string]RepoResolved{}
	for _, r := range l.Repos {
		m["aktion-"+r.ServiceID] = r
	}
	return m
}

func configuredRunnerImages(l Loaded) map[string]bool {
	m := map[string]bool{"ghcr.io/actions/actions-runner:latest": true}
	for _, r := range l.Repos {
		if r.Image != "" {
			m[r.Image] = true
		}
	}
	return m
}

func isRunnerImage(images map[string]bool, image string) bool {
	if images[image] {
		return true
	}
	return strings.HasPrefix(image, "ghcr.io/actions/actions-runner:")
}

func dockerLabel(labels, key string) string {
	for _, item := range strings.Split(labels, ",") {
		parts := strings.SplitN(strings.TrimSpace(item), "=", 2)
		if len(parts) == 2 && parts[0] == key {
			return parts[1]
		}
	}
	return ""
}

func logField(line, key string) string {
	re := regexp.MustCompile(`(?:^|\s)(?:[A-Za-z0-9_-]+\.)?` + regexp.QuoteMeta(key) + `=("(?:\\.|[^"])*"|\S+)`)
	m := re.FindStringSubmatch(line)
	if len(m) < 2 {
		return ""
	}
	v := m[1]
	if strings.HasPrefix(v, `"`) {
		if u, err := strconv.Unquote(v); err == nil {
			return u
		}
	}
	return v
}

func collectActiveJobs(l Loaded) map[string]activeJob {
	jobs := map[string]activeJob{}
	for _, r := range l.Repos {
		containerName := "aktion-" + r.ServiceID
		out, err := combinedOutput("", "docker", "logs", "--tail", "2000", containerName)
		if err != nil && len(out) == 0 {
			continue
		}
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, `msg="Job started"`) {
				runner := logField(line, "runnerName")
				if runner == "" {
					continue
				}
				started, _ := time.Parse(time.RFC3339Nano, logField(line, "time"))
				runID, _ := strconv.ParseInt(logField(line, "workflowRunId"), 10, 64)
				repo := logField(line, "repository")
				if repo == "" {
					owner, name, _ := parseGitHubRepoURL(r.URL)
					repo = owner + "/" + name
				}
				jobs[runner] = activeJob{
					Repository:  repo,
					JobID:       logField(line, "jobId"),
					JobName:     logField(line, "jobDisplayName"),
					WorkflowRef: logField(line, "jobWorkflowRef"),
					WorkflowRun: runID,
					StartedAt:   started,
				}
			}
			if strings.Contains(line, `msg="Job completed"`) {
				if runner := logField(line, "runnerName"); runner != "" {
					delete(jobs, runner)
				}
			}
		}
	}
	return jobs
}

func collectRepoDemand(l Loaded) map[string]repoDemand {
	out := map[string]repoDemand{}
	for _, r := range l.Repos {
		containerName := "aktion-" + r.ServiceID
		logs, err := combinedOutput("", "docker", "logs", "--tail", "400", containerName)
		if err != nil && len(logs) == 0 {
			continue
		}
		owner, repo, _ := parseGitHubRepoURL(r.URL)
		key := owner + "/" + repo
		for _, line := range strings.Split(string(logs), "\n") {
			if !strings.Contains(line, `msg="Scale demand"`) {
				continue
			}
			assigned, _ := strconv.Atoi(logField(line, "assignedJobs"))
			out[key] = repoDemand{Assigned: assigned}
		}	}
	return out
}

func filterJobsToRunningContainers(jobs map[string]activeJob, dockerRows []dockerRow, images map[string]bool) map[string]activeJob {
	running := map[string]bool{}
	for _, d := range dockerRows {
		if isRunnerImage(images, d.Image) && strings.EqualFold(d.State, "running") {
			running[d.Names] = true
		}
	}
	filtered := map[string]activeJob{}
	for runner, job := range jobs {
		if running[runner] {
			filtered[runner] = job
		}
	}
	return filtered
}

func workflowShort(ref string) string {
	if ref == "" {
		return "—"
	}
	if i := strings.Index(ref, "/.github/workflows/"); i >= 0 {
		v := ref[i+len("/.github/workflows/"):]
		if at := strings.Index(v, "@"); at >= 0 {
			v = v[:at]
		}
		return v
	}
	if at := strings.Index(ref, "@"); at >= 0 {
		ref = ref[:at]
	}
	return ref
}

func elapsedSince(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}

func cmdStatus(l Loaded) int {
	rows, err := dockerPSAll()
	if err != nil {
		return fail(err)
	}
	byName := map[string]dockerRow{}
	for _, r := range rows {
		byName[r.Names] = r
	}

	images := configuredRunnerImages(l)
	jobs := filterJobsToRunningContainers(collectActiveJobs(l), rows, images)
	demand := collectRepoDemand(l)
	runnerCount := 0
	listenerCount := 0
	runnersPerRepo := map[string]int{}
	activePerRepo := map[string]int{}
	for _, d := range rows {
		if isRunnerImage(images, d.Image) && strings.EqualFold(d.State, "running") {
			runnerCount++
			repo := dockerLabel(d.Labels, "io.aktion.repository")
			if repo == "" {
				if j, ok := jobs[d.Names]; ok {
					repo = j.Repository
				}
			}
			if repo != "" {
				runnersPerRepo[repo]++
			}
		}
	}
	for _, j := range jobs {
		activePerRepo[j.Repository]++
	}
	pendingCount := 0
	for repo, d := range demand {
		pending := d.Assigned - activePerRepo[repo]
		if pending > 0 {
			pendingCount += pending
		}
	}
	for _, r := range l.Repos {
		if d, ok := byName["aktion-"+r.ServiceID]; ok {
			_, _, healthy := listenerDisplayState(d)
			if healthy {
				listenerCount++
			}
		}
	}

	ui.banner("aktion", "local GitHub Actions runner scale sets")
	fmt.Println()
	listenerTone := "good"
	if listenerCount != len(l.Repos) {
		listenerTone = "warn"
	}
	runnerTone := "muted"
	if runnerCount > 0 {
		runnerTone = "good"
	}
	jobTone := "muted"
	if len(jobs) > 0 {
		jobTone = "info"
	}
	pendingTone := "muted"
	if pendingCount > 0 {
		pendingTone = "warn"
	}
	fmt.Println(ui.metrics(
		metricCard{Label: "Listeners", Value: fmt.Sprintf("%d / %d", listenerCount, len(l.Repos)), Tone: listenerTone, Hint: "healthy"},
		metricCard{Label: "Runners", Value: strconv.Itoa(runnerCount), Tone: runnerTone, Hint: "ephemeral"},
		metricCard{Label: "Active jobs", Value: strconv.Itoa(len(jobs)), Tone: jobTone, Hint: "running now"},
		metricCard{Label: "Pending", Value: strconv.Itoa(pendingCount), Tone: pendingTone, Hint: "assigned, waiting"},
	))
	if proxyEnabled(l.Config.Proxy) {
		fmt.Println()
		proxyText := "http=" + redactProxyURL(l.Config.Proxy.HTTP) + "  https=" + redactProxyURL(l.Config.Proxy.HTTPS)
		fmt.Println(ui.panel("Proxy", ui.renderStyle("muted", proxyText), "info"))
	}

	fmt.Println()
	ui.section("Repositories")
	tableRows := make([][]tableCell, 0, len(l.Repos))
	staleHint := false
	for _, r := range l.Repos {
		state, stateStyle := "STOPPED", "bad"
		if d, ok := byName["aktion-"+r.ServiceID]; ok {
			state, stateStyle, _ = listenerDisplayState(d)
			if state != "RUNNING" {
				if last := lastContainerError(d.Names); strings.Contains(last, "RunnerScaleSetExistsException") {
					staleHint = true
				}
			}
		}
		owner, repo, _ := parseGitHubRepoURL(r.URL)
		full := owner + "/" + repo
		runnerSummary := fmt.Sprintf("%d", runnersPerRepo[full])
		if activePerRepo[full] > 0 {
			runnerSummary = fmt.Sprintf("%d (%d busy)", runnersPerRepo[full], activePerRepo[full])
		}
		jobSummary := fmt.Sprintf("%d running", activePerRepo[full])
		if d, ok := demand[full]; ok {
			pending := d.Assigned - activePerRepo[full]
			if pending < 0 {
				pending = 0
			}
			jobSummary = fmt.Sprintf("%d run / %d wait", activePerRepo[full], pending)
		}
		tableRows = append(tableRows, []tableCell{linkedAccentCell(full, repoWebURL(full)), infoCell(r.ScaleSet), styledCell(state, stateStyle), cell(fmt.Sprintf("%d → %s", r.Min, formatMaxRunners(r.Max))), cell(runnerSummary), linkedCell(jobSummary, "", repoActionsURL(full))})
	}
	fmt.Println(ui.table([]string{"REPOSITORY", "SCALE SET", "LISTENER", "CAPACITY", "RUNNERS", "JOBS"}, tableRows))
	if staleHint {
		fmt.Println()
		fmt.Println(ui.panel("Stale scale set detected", ui.renderStyle("warn", "A listener is crash-looping because the remote scale set already exists. Run: aktion cleanup"), "warn"))
	}

	fmt.Println()
	ui.section("Runners / jobs")
	if runnerCount == 0 {
		fmt.Println(ui.panel("Scale to zero", ui.renderStyle("muted", "No ephemeral runners are active. A container will be created when GitHub assigns a job."), "info"))
	} else {
		printRunners(l, rows, jobs)
	}
	return 0
}

func printRepos(l Loaded) {
	ui.banner("Repositories", fmt.Sprintf("%d enabled scale set(s)", len(l.Repos)))
	fmt.Println()
	rows := make([][]tableCell, 0, len(l.Repos))
	for _, r := range l.Repos {
		featureText := "base"
		if len(r.Features) > 0 {
			featureText = strings.Join(r.Features, ",")
		}
		rows = append(rows, []tableCell{
			linkedAccentCell(r.ServiceID, strings.TrimRight(r.URL, "/")),
			linkedCell(strings.TrimRight(r.URL, "/"), "", strings.TrimRight(r.URL, "/")),
			infoCell(r.ScaleSet),
			cell(fmt.Sprintf("%d → %d", r.Min, r.Max)),
			mutedCell(featureText),
			mutedCell(strconv.FormatInt(r.Installation, 10)),
		})
	}
	fmt.Println(ui.table([]string{"ID", "REPOSITORY", "SCALE SET", "CAPACITY", "FEATURES", "INSTALLATION"}, rows))
}

func printRunners(l Loaded, dockerRows []dockerRow, jobs map[string]activeJob) {
	images := configuredRunnerImages(l)
	rows := make([][]tableCell, 0)
	for _, d := range dockerRows {
		if !isRunnerImage(images, d.Image) || !strings.EqualFold(d.State, "running") {
			continue
		}
		repo := dockerLabel(d.Labels, "io.aktion.repository")
		jobName, workflow, elapsed, state := "—", "—", "—", "IDLE"
		stateStyle := "muted"
		runTarget := ""
		workflowTarget := ""
		if j, ok := jobs[d.Names]; ok {
			if repo == "" {
				repo = j.Repository
			}
			jobName = j.JobName
			if jobName == "" {
				jobName = j.JobID
			}
			workflow = workflowShort(j.WorkflowRef)
			elapsed = elapsedSince(j.StartedAt)
			state = "RUNNING"
			stateStyle = "good"
			runTarget = workflowRunURL(repo, j.WorkflowRun)
			workflowTarget = workflowFileURL(repo, j.WorkflowRef)
			if workflowTarget == "" {
				workflowTarget = runTarget
			}
		}
		if repo == "" {
			repo = "unknown"
		}
		runnerTarget := runTarget
		if runnerTarget == "" {
			runnerTarget = repoActionsURL(repo)
		}
		rows = append(rows, []tableCell{
			linkedAccentCell(repo, repoWebURL(repo)),
			linkedInfoCell(d.Names, runnerTarget),
			styledCell(state, stateStyle),
			linkedCell(jobName, "", runTarget),
			linkedMutedCell(workflow, workflowTarget),
			cell(elapsed),
		})
	}
	if len(rows) == 0 {
		fmt.Println(ui.panel("Scale to zero", ui.renderStyle("muted", "No active runner containers."), "info"))
		return
	}
	fmt.Println(ui.table([]string{"REPOSITORY", "RUNNER", "STATE", "JOB", "WORKFLOW", "ELAPSED"}, rows))
}

func cmdStats(l Loaded) int {
	out, err := commandOutput("", "docker", "stats", "--no-stream", "--format", "{{json .}}")
	if err != nil {
		return fail(err)
	}
	listeners := listenerMap(l)
	images := configuredRunnerImages(l)
	ps, err := dockerPS()
	if err != nil {
		return fail(err)
	}
	imageByName := map[string]string{}
	for _, d := range ps {
		imageByName[d.Names] = d.Image
	}
	rows := make([][]tableCell, 0)
	for _, line := range bytes.Split(bytes.TrimSpace(out), []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var s dockerStat
		if json.Unmarshal(line, &s) != nil {
			continue
		}
		typ := ""
		typStyle := ""
		if _, ok := listeners[s.Name]; ok {
			typ = "listener"
			typStyle = "info"
		}
		if isRunnerImage(images, imageByName[s.Name]) {
			typ = "runner"
			typStyle = "good"
		}
		if typ == "" {
			continue
		}
		cpu := parsePercent(s.CPUPerc)
		mem := parsePercent(s.MemPerc)
		cpuTone := "info"
		if cpu >= 80 {
			cpuTone = "bad"
		} else if cpu >= 50 {
			cpuTone = "warn"
		}
		memTone := "info"
		if mem >= 80 {
			memTone = "bad"
		} else if mem >= 50 {
			memTone = "warn"
		}
		rows = append(rows, []tableCell{
			accentCell(s.Name),
			styledCell(typ, typStyle),
			cell(fmt.Sprintf("%-7s %s", s.CPUPerc, ui.progressBar(cpu, 10, cpuTone))),
			cell(s.MemUsage),
			cell(fmt.Sprintf("%-7s %s", s.MemPerc, ui.progressBar(mem, 10, memTone))),
			mutedCell(s.NetIO),
		})
	}
	ui.banner("Docker resources", "one-shot container snapshot")
	fmt.Println()
	if len(rows) == 0 {
		fmt.Println(ui.panel("No matching containers", ui.renderStyle("muted", "No scale-set listener or runner containers are currently visible to Docker."), "info"))
		return 0
	}
	fmt.Println(ui.table([]string{"CONTAINER", "TYPE", "CPU", "MEMORY", "MEM %", "NET I/O"}, rows))
	return 0
}

func parsePercent(v string) float64 {
	v = strings.TrimSpace(strings.TrimSuffix(v, "%"))
	f, _ := strconv.ParseFloat(v, 64)
	return f
}

func findRepoService(l Loaded, id string) (string, error) {
	sid := normalizeID(id)
	for _, r := range l.Repos {
		if r.ServiceID == sid || strings.EqualFold(r.ID, id) {
			return r.ServiceID, nil
		}
	}
	return "", fmt.Errorf("unknown enabled repository id %q", id)
}

func cmdDoctor(l Loaded) int {
	failures := 0
	type result struct {
		ok     bool
		group  string
		name   string
		detail string
	}
	var results []result
	checks := func(ok bool, group, name, detail string) {
		if !ok {
			failures++
		}
		results = append(results, result{ok: ok, group: group, name: name, detail: detail})
	}

	checks(true, "Local", "config.json", fmt.Sprintf("%d enabled repo(s)", len(l.Repos)))
	key, err := loadRSAKey(l.KeyPath)
	checks(err == nil, "Local", "GitHub App private key", errorDetail(err, filepath.Base(l.KeyPath)))

	if err := ensureDocker(); err != nil {
		checks(false, "Docker", "Docker Engine", err.Error())
	} else {
		checks(true, "Docker", "Docker Engine", "reachable")
		if out, e := commandOutput(l.Root, "docker", "compose", "version", "--short"); e != nil {
			checks(false, "Docker", "Docker Compose", e.Error())
		} else {
			checks(true, "Docker", "Docker Compose", strings.TrimSpace(string(out)))
		}
		if rows, e := dockerPSAll(); e == nil {
			byName := map[string]dockerRow{}
			for _, d := range rows {
				byName[d.Names] = d
			}
			for _, r := range l.Repos {
				name := "aktion-" + r.ServiceID
				if d, ok := byName[name]; ok {
					state, _, healthy := listenerDisplayState(d)
					detail := d.Status
					if !healthy {
						if last := lastContainerError(name); last != "" {
							detail = last
							if strings.Contains(last, "RunnerScaleSetExistsException") {
								detail += "; run aktion cleanup"
							}
						}
					}
					checks(healthy, "Runtime", r.ServiceID+" listener", state+": "+detail)
				}
			}
		}
	}

	if key != nil {
		jwt, e := signAppJWT(l.Config.App.ClientID, key)
		if e != nil {
			checks(false, "GitHub", "GitHub App JWT", e.Error())
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			app, e := githubGET(ctx, "https://api.github.com/app", jwt)
			if e != nil {
				checks(false, "GitHub", "GitHub App identity", e.Error())
			} else {
				slug, _ := app["slug"].(string)
				client, _ := app["client_id"].(string)
				detail := slug
				if client != "" && client != l.Config.App.ClientID {
					detail += " (client ID mismatch)"
					checks(false, "GitHub", "GitHub App identity", detail)
				} else {
					checks(true, "GitHub", "GitHub App identity", detail)
				}
			}

			checkedInst := map[int64]bool{}
			for _, r := range l.Repos {
				if !checkedInst[r.Installation] {
					path := fmt.Sprintf("https://api.github.com/app/installations/%d", r.Installation)
					inst, e := githubGET(ctx, path, jwt)
					if e != nil {
						checks(false, "GitHub", fmt.Sprintf("Installation %d", r.Installation), e.Error())
					} else {
						perms, _ := inst["permissions"].(map[string]any)
						admin, _ := perms["administration"].(string)
						detail := "reachable"
						if admin != "" {
							detail += ", administration=" + admin
						}
						checks(admin == "write", "GitHub", fmt.Sprintf("Installation %d", r.Installation), detail)
					}
					checkedInst[r.Installation] = true
				}
				owner, repo, _ := parseGitHubRepoURL(r.URL)
				endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/installation", url.PathEscape(owner), url.PathEscape(repo))
				info, e := githubGET(ctx, endpoint, jwt)
				if e != nil {
					checks(false, "Repositories", owner+"/"+repo, "App is not installed or repo is inaccessible: "+e.Error())
				} else {
					got := int64(numberAsFloat(info["id"]))
					if got != r.Installation {
						checks(false, "Repositories", owner+"/"+repo, fmt.Sprintf("installation=%d, config=%d", got, r.Installation))
					} else {
						checks(true, "Repositories", owner+"/"+repo, fmt.Sprintf("installation=%d", got))
					}
				}
			}
		}
	}

	ui.banner("Doctor", "configuration and connectivity checks")
	groups := []string{"Local", "Docker", "Runtime", "GitHub", "Repositories"}
	for _, group := range groups {
		var groupRows [][]tableCell
		for _, r := range results {
			if r.group != group {
				continue
			}
			state := badCell("FAIL")
			if r.ok {
				state = goodCell("PASS")
			}
			groupRows = append(groupRows, []tableCell{state, accentCell(r.name), mutedCell(r.detail)})
		}
		if len(groupRows) == 0 {
			continue
		}
		fmt.Println()
		ui.section(group)
		fmt.Println(ui.table([]string{"STATE", "CHECK", "DETAIL"}, groupRows))
	}
	fmt.Println()
	if failures > 0 {
		fmt.Println(ui.panel("Doctor failed", ui.renderStyle("bad", fmt.Sprintf("%d check(s) failed. Fix the items above and run doctor again.", failures)), "bad"))
		return 1
	}
	fmt.Println(ui.panel("All systems operational", ui.renderStyle("good", fmt.Sprintf("%d check(s) passed.", len(results))), "good"))
	return 0
}

func numberAsFloat(v any) float64 { f, _ := v.(float64); return f }
func errorDetail(err error, ok string) string {
	if err != nil {
		return err.Error()
	}
	return ok
}

func loadRSAKey(path string) (*rsa.PrivateKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, errors.New("PEM decode failed")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	v, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("unsupported private key: %w", err)
	}
	key, ok := v.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not RSA")
	}
	return key, nil
}

func signAppJWT(clientID string, key *rsa.PrivateKey) (string, error) {
	now := time.Now()
	header, _ := json.Marshal(map[string]any{"alg": "RS256", "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{
		"iat": now.Add(-60 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": clientID,
	})
	enc := base64.RawURLEncoding
	signing := enc.EncodeToString(header) + "." + enc.EncodeToString(claims)
	sum := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return signing + "." + enc.EncodeToString(sig), nil
}

func githubJSONRequest(ctx context.Context, method, endpoint, authorization string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(body); err != nil {
			return err
		}
		reader = &buf
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	req.Header.Set("User-Agent", "aktion-manager/"+version)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(data, &e)
		if e.Message == "" {
			e.Message = strings.TrimSpace(string(data))
		}
		return fmt.Errorf("GitHub API %s: %s", resp.Status, e.Message)
	}
	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return err
	}
	return nil
}

func installationAccessToken(ctx context.Context, installationID int64, appJWT string) (string, error) {
	endpoint := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)
	var token githubAccessToken
	if err := githubJSONRequest(ctx, http.MethodPost, endpoint, "Bearer "+appJWT, nil, &token); err != nil {
		return "", err
	}
	if token.Token == "" {
		return "", errors.New("GitHub App installation access token response was empty")
	}
	return token.Token, nil
}

func repoRunnerRegistrationToken(ctx context.Context, repoURL, installationToken string) (string, error) {
	owner, repo, err := parseGitHubRepoURL(repoURL)
	if err != nil {
		return "", err
	}
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runners/registration-token", url.PathEscape(owner), url.PathEscape(repo))
	var token runnerRegistrationToken
	if err := githubJSONRequest(ctx, http.MethodPost, endpoint, "Bearer "+installationToken, nil, &token); err != nil {
		return "", err
	}
	if token.Token == "" {
		return "", errors.New("runner registration token response was empty")
	}
	return token.Token, nil
}

func actionsAdmin(ctx context.Context, repoURL, registrationToken string) (actionsAdminConnection, error) {
	var conn actionsAdminConnection
	body := map[string]any{
		"url":          strings.TrimRight(repoURL, "/"),
		"runner_event": "register",
	}
	if err := githubJSONRequest(ctx, http.MethodPost, "https://api.github.com/actions/runner-registration", "RemoteAuth "+registrationToken, body, &conn); err != nil {
		return conn, err
	}
	if conn.URL == "" || conn.Token == "" {
		return conn, errors.New("Actions admin connection response is missing url or token")
	}
	return conn, nil
}

func actionsServiceJSON(ctx context.Context, method, baseURL, path, adminToken string, query url.Values, out any) error {
	endpoint, err := url.JoinPath(baseURL, path)
	if err != nil {
		return err
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	q := u.Query()
	for key, values := range query {
		for _, value := range values {
			q.Add(key, value)
		}
	}
	if q.Get("api-version") == "" {
		q.Set("api-version", "6.0-preview")
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("User-Agent", "aktion-manager/"+version)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Actions service %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

type scaleSetAdminClient struct {
	BaseURL string
	Token   string
}

func newScaleSetAdminClient(ctx context.Context, r RepoResolved, appJWT string, installationTokens map[int64]string) (scaleSetAdminClient, error) {
	installationToken := installationTokens[r.Installation]
	if installationToken == "" {
		var err error
		installationToken, err = installationAccessToken(ctx, r.Installation, appJWT)
		if err != nil {
			return scaleSetAdminClient{}, fmt.Errorf("installation %d access token: %w", r.Installation, err)
		}
		installationTokens[r.Installation] = installationToken
	}
	registrationToken, err := repoRunnerRegistrationToken(ctx, r.URL, installationToken)
	if err != nil {
		return scaleSetAdminClient{}, fmt.Errorf("runner registration token: %w", err)
	}
	conn, err := actionsAdmin(ctx, r.URL, registrationToken)
	if err != nil {
		return scaleSetAdminClient{}, fmt.Errorf("Actions admin connection: %w", err)
	}
	return scaleSetAdminClient{BaseURL: conn.URL, Token: conn.Token}, nil
}

func (c scaleSetAdminClient) getScaleSet(ctx context.Context, name string) (*remoteScaleSet, error) {
	q := url.Values{}
	q.Set("runnerGroupId", "1")
	q.Set("name", name)
	var list remoteScaleSetList
	if err := actionsServiceJSON(ctx, http.MethodGet, c.BaseURL, "_apis/runtime/runnerscalesets", c.Token, q, &list); err != nil {
		return nil, err
	}
	switch list.Count {
	case 0:
		return nil, nil
	case 1:
		return &list.Value[0], nil
	default:
		return nil, fmt.Errorf("multiple scale sets named %q found in Default runner group", name)
	}
}

func (c scaleSetAdminClient) deleteScaleSet(ctx context.Context, id int) error {
	return actionsServiceJSON(ctx, http.MethodDelete, c.BaseURL, fmt.Sprintf("_apis/runtime/runnerscalesets/%d", id), c.Token, nil, nil)
}

func appJWTForLoaded(l Loaded) (string, error) {
	key, err := loadRSAKey(l.KeyPath)
	if err != nil {
		return "", err
	}
	return signAppJWT(l.Config.App.ClientID, key)
}

func cmdScaleSets(l Loaded) int {
	appJWT, err := appJWTForLoaded(l)
	if err != nil {
		return fail(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	installationTokens := map[int64]string{}
	rows := make([][]tableCell, 0, len(l.Repos))
	failures := 0
	for _, r := range l.Repos {
		owner, repo, _ := parseGitHubRepoURL(r.URL)
		admin, err := newScaleSetAdminClient(ctx, r, appJWT, installationTokens)
		if err != nil {
			failures++
			rows = append(rows, []tableCell{linkedAccentCell(owner+"/"+repo, repoWebURL(owner+"/"+repo)), infoCell(r.ScaleSet), badCell("ERROR"), mutedCell(err.Error())})
			continue
		}
		ss, err := admin.getScaleSet(ctx, r.ScaleSet)
		if err != nil {
			failures++
			rows = append(rows, []tableCell{linkedAccentCell(owner+"/"+repo, repoWebURL(owner+"/"+repo)), infoCell(r.ScaleSet), badCell("ERROR"), mutedCell(err.Error())})
			continue
		}
		if ss == nil {
			rows = append(rows, []tableCell{linkedAccentCell(owner+"/"+repo, repoWebURL(owner+"/"+repo)), infoCell(r.ScaleSet), mutedCell("MISSING"), mutedCell("not registered")})
			continue
		}
		rows = append(rows, []tableCell{linkedAccentCell(owner+"/"+repo, repoWebURL(owner+"/"+repo)), infoCell(ss.Name), goodCell("FOUND"), cell(fmt.Sprintf("id=%d group=%s", ss.ID, strOr(ss.RunnerGroupName, "Default")))})
	}
	ui.banner("Remote scale sets", "GitHub Actions service state")
	fmt.Println()
	fmt.Println(ui.table([]string{"REPOSITORY", "SCALE SET", "STATE", "DETAIL"}, rows))
	if failures > 0 {
		return 1
	}
	return 0
}

func cmdCleanup(l Loaded, args []string) int {
	force := false
	noStart := false
	for _, arg := range args {
		switch arg {
		case "--force", "-f":
			force = true
		case "--no-start":
			noStart = true
		default:
			return fail(fmt.Errorf("unknown cleanup option %q (supported: --force, --no-start)", arg))
		}
	}
	if err := ensureDocker(); err != nil {
		return fail(err)
	}
	running, err := dockerPS()
	if err != nil {
		return fail(err)
	}
	images := configuredRunnerImages(l)
	activeRunners := 0
	for _, d := range running {
		if isRunnerImage(images, d.Image) {
			activeRunners++
		}
	}
	if activeRunners > 0 && !force {
		return fail(fmt.Errorf("refusing cleanup while %d ephemeral runner(s) are active; wait for jobs to finish or use cleanup --force", activeRunners))
	}

	p, err := renderCompose(l)
	if err != nil {
		return fail(err)
	}
	ui.banner("Cleanup", "remove stale GitHub runner scale sets")
	fmt.Println()
	fmt.Println(ui.renderStyle("muted", "Stopping local listeners before remote cleanup..."))
	if code := runStreaming(l.Root, "docker", "compose", "-f", p, "down", "--remove-orphans"); code != 0 {
		return code
	}

	appJWT, err := appJWTForLoaded(l)
	if err != nil {
		return fail(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	installationTokens := map[int64]string{}
	rows := make([][]tableCell, 0, len(l.Repos))
	failures := 0
	for _, r := range l.Repos {
		owner, repo, _ := parseGitHubRepoURL(r.URL)
		admin, err := newScaleSetAdminClient(ctx, r, appJWT, installationTokens)
		if err != nil {
			failures++
			rows = append(rows, []tableCell{linkedAccentCell(owner+"/"+repo, repoWebURL(owner+"/"+repo)), infoCell(r.ScaleSet), badCell("ERROR"), mutedCell(err.Error())})
			continue
		}
		ss, err := admin.getScaleSet(ctx, r.ScaleSet)
		if err != nil {
			failures++
			rows = append(rows, []tableCell{linkedAccentCell(owner+"/"+repo, repoWebURL(owner+"/"+repo)), infoCell(r.ScaleSet), badCell("ERROR"), mutedCell(err.Error())})
			continue
		}
		if ss == nil {
			rows = append(rows, []tableCell{linkedAccentCell(owner+"/"+repo, repoWebURL(owner+"/"+repo)), infoCell(r.ScaleSet), mutedCell("ABSENT"), mutedCell("nothing to delete")})
			continue
		}
		if err := admin.deleteScaleSet(ctx, ss.ID); err != nil {
			failures++
			rows = append(rows, []tableCell{linkedAccentCell(owner+"/"+repo, repoWebURL(owner+"/"+repo)), infoCell(r.ScaleSet), badCell("ERROR"), mutedCell(fmt.Sprintf("id=%d: %v", ss.ID, err))})
			continue
		}
		rows = append(rows, []tableCell{linkedAccentCell(owner+"/"+repo, repoWebURL(owner+"/"+repo)), infoCell(r.ScaleSet), goodCell("DELETED"), cell(fmt.Sprintf("id=%d", ss.ID))})
	}
	fmt.Println()
	fmt.Println(ui.table([]string{"REPOSITORY", "SCALE SET", "RESULT", "DETAIL"}, rows))
	if failures > 0 {
		fmt.Println()
		fmt.Println(ui.panel("Cleanup incomplete", ui.renderStyle("bad", fmt.Sprintf("%d repository cleanup(s) failed. Listeners were left stopped to avoid a crash-loop.", failures)), "bad"))
		return 1
	}
	if noStart {
		fmt.Println()
		fmt.Println(ui.panel("Cleanup complete", ui.renderStyle("good", "Remote scale sets were cleaned. Local listeners remain stopped because --no-start was requested."), "good"))
		return 0
	}
	fmt.Println()
	fmt.Println(ui.renderStyle("muted", "Starting listeners with the repaired SIGINT shutdown policy..."))
	if code := runStreaming(l.Root, "docker", "compose", "-f", p, "up", "-d", "--build"); code != 0 {
		return code
	}
	fmt.Println()
	fmt.Println(ui.panel("Cleanup complete", ui.renderStyle("good", "Stale scale sets were removed and listeners were restarted."), "good"))
	return 0
}

func githubGET(ctx context.Context, endpoint, jwt string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	req.Header.Set("User-Agent", "aktion-manager/"+version)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &e)
		if e.Message == "" {
			e.Message = strings.TrimSpace(string(body))
		}
		return nil, fmt.Errorf("GitHub API %s: %s", resp.Status, e.Message)
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func parseGitHubRepoURL(s string) (string, string, error) {
	u, err := url.Parse(strings.TrimRight(s, "/"))
	if err != nil {
		return "", "", err
	}
	if !strings.EqualFold(u.Scheme, "https") || !strings.EqualFold(u.Host, "github.com") {
		return "", "", errors.New("URL must look like https://github.com/OWNER/REPO")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("URL must look like https://github.com/OWNER/REPO")
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git"), nil
}

func repoWebURL(repository string) string {
	repository = strings.Trim(strings.TrimSpace(repository), "/")
	if repository == "" || strings.Count(repository, "/") != 1 {
		return ""
	}
	return "https://github.com/" + repository
}

func repoActionsURL(repository string) string {
	base := repoWebURL(repository)
	if base == "" {
		return ""
	}
	return base + "/actions"
}

func workflowRunURL(repository string, runID int64) string {
	if runID <= 0 {
		return repoActionsURL(repository)
	}
	base := repoWebURL(repository)
	if base == "" {
		return ""
	}
	return fmt.Sprintf("%s/actions/runs/%d", base, runID)
}

func workflowFileURL(repository, workflowRef string) string {
	if repository == "" || workflowRef == "" {
		return ""
	}
	prefix := repository + "/"
	ref := workflowRef
	if strings.HasPrefix(ref, prefix) {
		ref = strings.TrimPrefix(ref, prefix)
	}
	at := strings.LastIndex(ref, "@")
	if at < 0 {
		return ""
	}
	pathPart := strings.TrimPrefix(ref[:at], "/")
	gitRef := ref[at+1:]
	switch {
	case strings.HasPrefix(gitRef, "refs/heads/"):
		gitRef = strings.TrimPrefix(gitRef, "refs/heads/")
	case strings.HasPrefix(gitRef, "refs/tags/"):
		gitRef = strings.TrimPrefix(gitRef, "refs/tags/")
	default:
		return ""
	}
	if pathPart == "" || gitRef == "" {
		return ""
	}
	u := &url.URL{
		Scheme: "https",
		Host:   "github.com",
		Path:   "/" + repository + "/blob/" + gitRef + "/" + pathPart,
	}
	return u.String()
}

func fail(err error) int {
	if ui != nil {
		ui.errorLine(os.Stderr, "error: ", err.Error())
	} else {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	return 1
}