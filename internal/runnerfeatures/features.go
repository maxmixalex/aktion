package runnerfeatures

import (
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
)

const RuntimeLabel = "io.aktion.runner-feature-runtime"

var builtinIDs = map[string]bool{
	"build":   true,
	"native":  true,
	"glib":    true,
	"gtk4":    true,
	"ncurses": true,
	"lua54":   true,
}

//go:embed features/*/feature.json features/*/install.sh
var builtinFiles embed.FS

type OptionDefinition struct {
	Type        string
	Enum        []string
	Proposals   []string
	Default     any
	Description string
}

type Mount struct {
	Type     string
	Source   string
	Target   string
	External bool
}

type Manifest struct {
	ID               string
	Version          string
	Name             string
	Description      string
	DocumentationURL string
	LicenseURL       string
	Options          map[string]OptionDefinition
	ContainerEnv     map[string]string
	Privileged       bool
	Init             bool
	CapAdd           []string
	SecurityOpt      []string
	Entrypoint       string
	Mounts           []Mount
	DependsOn        map[string]json.RawMessage
	InstallsAfter    []string
	Deprecated       bool

	OnCreateCommand      json.RawMessage
	UpdateContentCommand json.RawMessage
	PostCreateCommand    json.RawMessage
	PostStartCommand     json.RawMessage
	PostAttachCommand    json.RawMessage
}

type FeatureFile struct {
	Path string
	Mode os.FileMode
	Data []byte
}

type FeaturePackage struct {
	Reference      string
	Identity       string
	Canonical      string
	ManifestDigest string
	Manifest       Manifest
	Files          []FeatureFile
}

type ResolvedFeature struct {
	Reference string
	Identity  string
	Canonical string
	Digest    string
	Manifest  Manifest
	Options   Options
	Files     []FeatureFile
	Builtin   bool
}

type RuntimeMount struct {
	Type   string
	Source string
	Target string
}

type RuntimeConfig struct {
	Init        bool
	Privileged  bool
	CapAdd      []string
	SecurityOpt []string
	Mounts      []RuntimeMount
	Entrypoints []string
}

func EncodeRuntime(runtime RuntimeConfig) (string, error) {
	data, err := json.Marshal(runtime)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func DecodeRuntime(encoded string) (RuntimeConfig, error) {
	if strings.TrimSpace(encoded) == "" {
		return RuntimeConfig{}, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("decode feature runtime label: %w", err)
	}
	var runtime RuntimeConfig
	if err := json.Unmarshal(data, &runtime); err != nil {
		return RuntimeConfig{}, fmt.Errorf("parse feature runtime label: %w", err)
	}
	return runtime, nil
}

type builtinManifest struct {
	ID            string
	Name          string
	Description   string
	DependsOn     []string
	InstallsAfter []string
}

type BuiltinFeature struct {
	Manifest Manifest
	Install  []byte
	Digest   string
}

func Catalog() (map[string]BuiltinFeature, error) {
	out := make(map[string]BuiltinFeature, len(builtinIDs))
	ids := make([]string, 0, len(builtinIDs))
	for id := range builtinIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		meta, err := fs.ReadFile(builtinFiles, "features/"+id+"/feature.json")
		if err != nil {
			return nil, err
		}
		install, err := fs.ReadFile(builtinFiles, "features/"+id+"/install.sh")
		if err != nil {
			return nil, err
		}
		var raw builtinManifest
		if err := json.Unmarshal(meta, &raw); err != nil {
			return nil, fmt.Errorf("builtin feature %s: %w", id, err)
		}
		if raw.ID != id || raw.ID == "" {
			return nil, fmt.Errorf("builtin feature directory %q has id %q", id, raw.ID)
		}
		depends := map[string]json.RawMessage{}
		for _, dep := range raw.DependsOn {
			depends["builtin:"+dep] = json.RawMessage("{}")
		}
		installsAfter := make([]string, 0, len(raw.InstallsAfter))
		for _, dep := range raw.InstallsAfter {
			installsAfter = append(installsAfter, "builtin:"+dep)
		}
		h := sha256.New()
		_, _ = h.Write(meta)
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(install)
		out[id] = BuiltinFeature{
			Manifest: Manifest{
				ID:            id,
				Version:       "builtin",
				Name:          raw.Name,
				Description:   raw.Description,
				DependsOn:     depends,
				InstallsAfter: installsAfter,
			},
			Install: install,
			Digest:  hex.EncodeToString(h.Sum(nil)),
		}
	}
	return out, nil
}

func loadBuiltin(ref string) (FeaturePackage, error) {
	normalized, err := NormalizeReference(ref)
	if err != nil {
		return FeaturePackage{}, err
	}
	id := strings.TrimPrefix(normalized, "builtin:")
	catalog, err := Catalog()
	if err != nil {
		return FeaturePackage{}, err
	}
	feature, ok := catalog[id]
	if !ok {
		return FeaturePackage{}, fmt.Errorf("unknown builtin feature %q", id)
	}
	return FeaturePackage{
		Reference: normalized,
		Identity:  normalized,
		Canonical: normalized + "@" + feature.Digest,
		Manifest:  feature.Manifest,
		Files: []FeatureFile{
			{Path: "install.sh", Mode: 0755, Data: feature.Install},
		},
		ManifestDigest: feature.Digest,
	}, nil
}

func hasLifecycleHook(m Manifest) bool {
	return nonEmptyJSON(m.OnCreateCommand) ||
		nonEmptyJSON(m.UpdateContentCommand) ||
		nonEmptyJSON(m.PostCreateCommand) ||
		nonEmptyJSON(m.PostStartCommand) ||
		nonEmptyJSON(m.PostAttachCommand)
}

func nonEmptyJSON(raw json.RawMessage) bool {
	v := strings.TrimSpace(string(raw))
	return v != "" && v != "null"
}
