package runnerfeatures

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type FeatureFetcher interface {
	FetchFeature(context.Context, string) (FeaturePackage, error)
}

type Resolver struct {
	OCI FeatureFetcher
}

func NewResolver(cacheDir string) *Resolver {
	return &Resolver{OCI: NewOCIClient(cacheDir)}
}

type Plan struct {
	BaseImage  string
	Image      string
	Features   []ResolvedFeature
	Runtime    RuntimeConfig
	Dockerfile string
	Warnings   []string
}

func (r *Resolver) ResolvePlan(ctx context.Context, baseImage string, selections Selections) (Plan, error) {
	normalized, err := NormalizeSelections(selections)
	if err != nil {
		return Plan{}, err
	}
	imageName, baseImage, err := ImageName(baseImage, normalized)
	if err != nil {
		return Plan{}, err
	}
	if len(normalized) == 0 {
		return Plan{BaseImage: baseImage, Image: imageName}, nil
	}

	type node struct {
		feature ResolvedFeature
	}
	nodes := map[string]*node{}
	edges := map[string]map[string]bool{}

	var addFeature func(string, Options) (string, error)
	addFeature = func(ref string, provided Options) (string, error) {
		normalizedRef, err := NormalizeReference(ref)
		if err != nil {
			return "", err
		}

		var pkg FeaturePackage
		if strings.HasPrefix(normalizedRef, "builtin:") {
			pkg, err = loadBuiltin(normalizedRef)
		} else {
			if r.OCI == nil {
				return "", fmt.Errorf("OCI resolver is unavailable for %s", normalizedRef)
			}
			pkg, err = r.OCI.FetchFeature(ctx, normalizedRef)
		}
		if err != nil {
			return "", err
		}

		resolvedOptions, err := resolveOptions(pkg.Manifest, provided)
		if err != nil {
			return "", fmt.Errorf("%s: %w", normalizedRef, err)
		}
		feature := ResolvedFeature{
			Reference: normalizedRef,
			Identity:  pkg.Identity,
			Canonical: pkg.Canonical,
			Digest:    pkg.ManifestDigest,
			Manifest:  pkg.Manifest,
			Options:   resolvedOptions,
			Files:     pkg.Files,
			Builtin:   strings.HasPrefix(normalizedRef, "builtin:"),
		}

		if existing, ok := nodes[feature.Identity]; ok {
			sameOptions, err := optionsEqual(existing.feature.Options, feature.Options)
			if err != nil {
				return "", err
			}
			if existing.feature.Canonical != feature.Canonical || !sameOptions {
				return "", fmt.Errorf(
					"feature %s was requested more than once with incompatible version/options (%s vs %s)",
					feature.Identity,
					existing.feature.Reference,
					feature.Reference,
				)
			}
			return feature.Identity, nil
		}
		nodes[feature.Identity] = &node{feature: feature}
		edges[feature.Identity] = map[string]bool{}

		depRefs := make([]string, 0, len(feature.Manifest.DependsOn))
		for depRef := range feature.Manifest.DependsOn {
			depRefs = append(depRefs, depRef)
		}
		sort.Strings(depRefs)
		for _, depRef := range depRefs {
			depOptions, include, err := parseSelectionValue(feature.Manifest.DependsOn[depRef])
			if err != nil {
				return "", fmt.Errorf("%s dependsOn %s: %w", feature.Reference, depRef, err)
			}
			if !include {
				return "", fmt.Errorf("%s declares disabled hard dependency %s", feature.Reference, depRef)
			}
			depIdentity, err := addFeature(depRef, depOptions)
			if err != nil {
				return "", fmt.Errorf("%s dependsOn %s: %w", feature.Reference, depRef, err)
			}
			edges[depIdentity][feature.Identity] = true
		}
		return feature.Identity, nil
	}

	for _, ref := range SelectionKeys(normalized) {
		if _, err := addFeature(ref, normalized[ref]); err != nil {
			return Plan{}, err
		}
	}

	for identity, n := range nodes {
		for _, after := range n.feature.Manifest.InstallsAfter {
			afterIdentity, err := featureIdentity(after)
			if err != nil {
				return Plan{}, fmt.Errorf("%s installsAfter %s: %w", n.feature.Reference, after, err)
			}
			if _, ok := nodes[afterIdentity]; ok && afterIdentity != identity {
				edges[afterIdentity][identity] = true
			}
		}
	}

	indegree := make(map[string]int, len(nodes))
	for identity := range nodes {
		indegree[identity] = 0
	}
	for _, tos := range edges {
		for to := range tos {
			indegree[to]++
		}
	}

	ordered := make([]ResolvedFeature, 0, len(nodes))
	processed := map[string]bool{}
	for len(ordered) < len(nodes) {
		ready := make([]string, 0)
		for identity, degree := range indegree {
			if degree == 0 && !processed[identity] {
				ready = append(ready, identity)
			}
		}
		if len(ready) == 0 {
			return Plan{}, fmt.Errorf("runner feature dependency cycle detected")
		}
		sort.Slice(ready, func(i, j int) bool {
			return nodes[ready[i]].feature.Reference < nodes[ready[j]].feature.Reference
		})
		identity := ready[0]
		processed[identity] = true
		ordered = append(ordered, nodes[identity].feature)
		for to := range edges[identity] {
			indegree[to]--
		}
	}

	runtime, warnings := mergeRuntime(ordered)
	dockerfile, err := buildDockerfile(baseImage, ordered, runtime)
	if err != nil {
		return Plan{}, err
	}
	return Plan{
		BaseImage:  baseImage,
		Image:      imageName,
		Features:   ordered,
		Runtime:    runtime,
		Dockerfile: dockerfile,
		Warnings:   warnings,
	}, nil
}

func featureIdentity(ref string) (string, error) {
	ref = strings.ToLower(strings.TrimSpace(ref))
	if strings.HasPrefix(ref, "builtin:") {
		normalized, err := NormalizeReference(ref)
		if err != nil {
			return "", err
		}
		return normalized, nil
	}
	oci, err := ParseOCIReference(ref)
	if err != nil {
		return "", err
	}
	return oci.Identity(), nil
}

func resolveOptions(manifest Manifest, provided Options) (Options, error) {
	resolved := make(Options, len(manifest.Options))
	for key, definition := range manifest.Options {
		if definition.Default != nil {
			switch value := definition.Default.(type) {
			case string, bool:
				resolved[key] = value
			default:
				return nil, fmt.Errorf("option %q has unsupported default type", key)
			}
		}
	}
	for key, value := range provided {
		definition, ok := manifest.Options[key]
		if !ok {
			return nil, fmt.Errorf("unknown option %q", key)
		}
		switch definition.Type {
		case "string":
			s, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("option %q must be a string", key)
			}
			if len(definition.Enum) > 0 && !contains(definition.Enum, s) {
				return nil, fmt.Errorf("option %q must be one of %s", key, strings.Join(definition.Enum, ", "))
			}
			resolved[key] = s
		case "boolean":
			b, ok := value.(bool)
			if !ok {
				return nil, fmt.Errorf("option %q must be a boolean", key)
			}
			resolved[key] = b
		default:
			return nil, fmt.Errorf("option %q has unsupported type %q", key, definition.Type)
		}
	}
	return resolved, nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func optionsEqual(a, b Options) (bool, error) {
	aj, err := json.Marshal(a)
	if err != nil {
		return false, err
	}
	bj, err := json.Marshal(b)
	if err != nil {
		return false, err
	}
	return string(aj) == string(bj), nil
}

func mergeRuntime(features []ResolvedFeature) (RuntimeConfig, []string) {
	var runtime RuntimeConfig
	caps := map[string]bool{}
	security := map[string]bool{}
	mountIndex := map[string]int{}
	var warnings []string

	for _, feature := range features {
		m := feature.Manifest
		runtime.Init = runtime.Init || m.Init
		runtime.Privileged = runtime.Privileged || m.Privileged

		for _, cap := range m.CapAdd {
			if !caps[cap] {
				caps[cap] = true
				runtime.CapAdd = append(runtime.CapAdd, cap)
			}
		}
		for _, opt := range m.SecurityOpt {
			if !security[opt] {
				security[opt] = true
				runtime.SecurityOpt = append(runtime.SecurityOpt, opt)
			}
		}
		for _, mount := range m.Mounts {
			rm := RuntimeMount{Type: mount.Type, Source: mount.Source, Target: mount.Target}
			if index, exists := mountIndex[mount.Target]; exists {
				runtime.Mounts[index] = rm
			} else {
				mountIndex[mount.Target] = len(runtime.Mounts)
				runtime.Mounts = append(runtime.Mounts, rm)
			}
		}
		if m.Entrypoint != "" {
			runtime.Entrypoints = append(runtime.Entrypoints, m.Entrypoint)
		}
		if m.Deprecated {
			warnings = append(warnings, fmt.Sprintf("%s is deprecated by its publisher", feature.Reference))
		}
		if hasLifecycleHook(m) {
			warnings = append(warnings, fmt.Sprintf("%s declares Dev Container lifecycle hooks; Aktion ignores lifecycle hooks because GitHub job workspaces are created at runtime", feature.Reference))
		}
	}
	return runtime, warnings
}

func buildDockerfile(baseImage string, features []ResolvedFeature, runtime RuntimeConfig) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "FROM %s AS aktion-base\n", baseImage)
	b.WriteString("USER root\n")
	previous := "aktion-base"

	for i, feature := range features {
		stage := fmt.Sprintf("aktion-feature-%02d-%s", i+1, sanitizeStage(feature.Manifest.ID))
		contextDir := fmt.Sprintf("%02d-%s", i+1, sanitizeStage(feature.Manifest.ID))
		fmt.Fprintf(&b, "\nFROM %s AS %s\n", previous, stage)
		b.WriteString("USER root\n")
		fmt.Fprintf(&b, "COPY features/%s/ /tmp/aktion-feature/\n", contextDir)
		b.WriteString("RUN chmod +x /tmp/aktion-feature/install.sh \\\n")
		b.WriteString("    && cd /tmp/aktion-feature \\\n")
		b.WriteString("    && ")
		for _, assignment := range featureInstallEnv(feature.Options) {
			b.WriteString(assignment)
			b.WriteByte(' ')
		}
		b.WriteString("./install.sh \\\n")
		b.WriteString("    && rm -rf /tmp/aktion-feature\n")

		envKeys := make([]string, 0, len(feature.Manifest.ContainerEnv))
		for key := range feature.Manifest.ContainerEnv {
			envKeys = append(envKeys, key)
		}
		sort.Strings(envKeys)
		for _, key := range envKeys {
			if !validEnvName(key) {
				return "", fmt.Errorf("%s has invalid containerEnv key %q", feature.Reference, key)
			}
			fmt.Fprintf(&b, "ENV %s=%s\n", key, strconv.Quote(feature.Manifest.ContainerEnv[key]))
		}
		previous = stage
	}

	encodedRuntime, err := EncodeRuntime(runtime)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(&b, "\nFROM %s AS final\n", previous)
	fmt.Fprintf(&b, "LABEL %s=%s\n", RuntimeLabel, strconv.Quote(encodedRuntime))
	b.WriteString("USER runner\n")
	return b.String(), nil
}

func featureInstallEnv(options Options) []string {
	env := []string{
		"_REMOTE_USER='runner'",
		"_CONTAINER_USER='runner'",
		"_REMOTE_USER_HOME='/home/runner'",
		"_CONTAINER_USER_HOME='/home/runner'",
	}
	keys := make([]string, 0, len(options))
	for key := range options {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		name := strings.ToUpper(key)
		var value string
		switch v := options[key].(type) {
		case bool:
			value = strconv.FormatBool(v)
		case string:
			value = v
		}
		env = append(env, name+"="+shellQuote(value))
	}
	return env
}

func validEnvName(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_' || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func sanitizeStage(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	if b.Len() == 0 {
		return "feature"
	}
	return b.String()
}

func WriteBuildContext(plan Plan, dir string) error {
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(plan.Dockerfile), 0644); err != nil {
		return err
	}
	for i, feature := range plan.Features {
		contextDir := fmt.Sprintf("%02d-%s", i+1, sanitizeStage(feature.Manifest.ID))
		root := filepath.Join(dir, "features", contextDir)
		for _, file := range feature.Files {
			clean := filepath.Clean(filepath.FromSlash(file.Path))
			if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				return fmt.Errorf("unsafe feature file path %q", file.Path)
			}
			target := filepath.Join(root, clean)
			rel, err := filepath.Rel(root, target)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return fmt.Errorf("unsafe feature file path %q", file.Path)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			mode := file.Mode
			if mode == 0 {
				mode = 0644
			}
			if err := os.WriteFile(target, file.Data, mode); err != nil {
				return err
			}
		}
	}
	return nil
}
