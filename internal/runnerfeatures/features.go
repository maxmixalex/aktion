package runnerfeatures

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

const RuntimeLabel = "io.aktion.runner-feature-runtime"

type builtinDefinition struct {
	Name          string
	Description   string
	DependsOn     []string
	InstallsAfter []string
}

var builtinDefinitions = map[string]builtinDefinition{
	"build":  {Name: "Build toolchain", Description: "Common C/C++ build toolchain"},
	"clang":  {Name: "Clang compiler", Description: "Clang C/C++ compiler from the distribution package", DependsOn: []string{"build"}},
	"lua54":  {Name: "Lua 5.4 development", Description: "Lua 5.4 development headers", DependsOn: []string{"build"}},
	"svn":    {Name: "Subversion client", Description: "Apache Subversion command-line client"},
}

//go:embed features/*/install.sh
var builtinFiles embed.FS

type OptionDefinition struct {
	Type        string
	Enum        []string
	Default     any
}

type Mount struct {
	Type     string
	Source   string
	Target   string
}

type Manifest struct {
	ID               string
	Version          string
	Name             string
	Description      string
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
	Identity  string
	Canonical string
	Manifest  Manifest
	Files     []FeatureFile
}

type ResolvedFeature struct {
	Reference string
	Identity  string
	Canonical string
	Manifest  Manifest
	Options   Options
	Files     []FeatureFile
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

type BuiltinFeature struct {
	Manifest Manifest
	Install  []byte
}

func Catalog() (map[string]BuiltinFeature, error) {
	out := make(map[string]BuiltinFeature, len(builtinDefinitions))
	ids := make([]string, 0, len(builtinDefinitions))
	for id := range builtinDefinitions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		definition := builtinDefinitions[id]
		install, err := builtinFiles.ReadFile("features/" + id + "/install.sh")
		if err != nil {
			return nil, err
		}
		depends := map[string]json.RawMessage{}
		for _, dep := range definition.DependsOn {
			depends["builtin:"+dep] = json.RawMessage("{}")
		}
		installsAfter := make([]string, 0, len(definition.InstallsAfter))
		for _, dep := range definition.InstallsAfter {
			installsAfter = append(installsAfter, "builtin:"+dep)
		}
		out[id] = BuiltinFeature{
			Manifest: Manifest{
				ID:            id,
				Version:       "builtin",
				Name:          definition.Name,
				Description:   definition.Description,
				DependsOn:     depends,
				InstallsAfter: installsAfter,
			},
			Install: install,
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
		Identity:  normalized,
		Canonical: normalized,
		Manifest:  feature.Manifest,
		Files: []FeatureFile{
			{Path: "install.sh", Mode: 0755, Data: feature.Install},
		},
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
