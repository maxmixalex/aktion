package runnerfeatures

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const DefaultBaseImage = "ghcr.io/actions/actions-runner:latest"

type Options map[string]any

// Selections mirrors the Dev Container "features" object: feature URI -> options.
// A string value is accepted as shorthand for {"version": "..."}, and true means
// default options. false omits the feature.
type Selections map[string]Options

func (s *Selections) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" {
		*s = nil
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		return fmt.Errorf("runnerFeatures now uses an object keyed by Feature URI, not the legacy string array")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("runnerFeatures must be an object keyed by feature URI: %w", err)
	}
	out := make(Selections, len(raw))
	for ref, value := range raw {
		options, include, err := parseSelectionValue(value)
		if err != nil {
			return fmt.Errorf("%s: %w", ref, err)
		}
		if include {
			out[ref] = options
		} else {
			// Preserve an explicit false until inheritance is resolved. Normal
			// selection normalization drops disabled entries.
			out[ref] = nil
		}
	}
	*s = out
	return nil
}

func parseSelectionValue(raw json.RawMessage) (Options, bool, error) {
	var scalar any
	if err := json.Unmarshal(raw, &scalar); err != nil {
		return nil, false, err
	}
	switch v := scalar.(type) {
	case bool:
		if !v {
			return nil, false, nil
		}
		return Options{}, true, nil
	case string:
		return Options{"version": v}, true, nil
	case map[string]any:
		out := make(Options, len(v))
		for key, value := range v {
			switch value.(type) {
			case string, bool:
				out[key] = value
			default:
				return nil, false, fmt.Errorf("option %q must be a string or boolean", key)
			}
		}
		return out, true, nil
	default:
		return nil, false, fmt.Errorf("feature value must be an object, string, or boolean")
	}
}

func CloneSelections(in Selections) Selections {
	if in == nil {
		return nil
	}
	out := make(Selections, len(in))
	for ref, opts := range in {
		copied := make(Options, len(opts))
		for key, value := range opts {
			copied[key] = value
		}
		out[ref] = copied
	}
	return out
}

func DefaultSelections() Selections {
	return Selections{
		"builtin:build": {},
		"builtin:native": {},
		"ghcr.io/devcontainers/features/common-utils:2": {
			"installZsh":               false,
			"configureZshAsDefaultShell": false,
			"installOhMyZsh":           false,
			"installOhMyZshConfig":     false,
			"upgradePackages":          false,
			"username":                 "none",
		},
		"ghcr.io/devcontainers/features/python:1": {
			"version":      "os-provided",
			"installTools": false,
		},
		"ghcr.io/devcontainers/features/git-lfs:1": {
			"autoPull": false,
		},
	}
}

func NormalizeSelections(in Selections) (Selections, error) {
	if in == nil {
		return nil, nil
	}
	out := make(Selections, len(in))
	for ref, opts := range in {
		if opts == nil {
			continue
		}
		normalized, err := NormalizeReference(ref)
		if err != nil {
			return nil, err
		}
		if _, exists := out[normalized]; exists {
			return nil, fmt.Errorf("duplicate feature after normalization: %q", normalized)
		}
		copied := make(Options, len(opts))
		for key, value := range opts {
			switch value.(type) {
			case string, bool:
				copied[key] = value
			default:
				return nil, fmt.Errorf("%s option %q must be a string or boolean", normalized, key)
			}
		}
		out[normalized] = copied
	}
	return out, nil
}

func NormalizeReference(ref string) (string, error) {
	ref = strings.ToLower(strings.TrimSpace(ref))
	if ref == "" {
		return "", fmt.Errorf("feature reference cannot be empty")
	}
	if strings.HasPrefix(ref, "builtin:") {
		id := strings.TrimPrefix(ref, "builtin:")
		if id == "" || strings.ContainsAny(id, "/:@") {
			return "", fmt.Errorf("invalid builtin feature reference %q", ref)
		}
		if _, ok := builtinIDs[id]; !ok {
			return "", fmt.Errorf("unknown builtin feature %q", id)
		}
		return "builtin:" + id, nil
	}
	oci, err := ParseOCIReference(ref)
	if err != nil {
		return "", err
	}
	return oci.String(), nil
}

func SelectionKeys(in Selections) []string {
	keys := make([]string, 0, len(in))
	for ref := range in {
		keys = append(keys, ref)
	}
	sort.Strings(keys)
	return keys
}

func ImageName(baseImage string, selections Selections) (string, string, error) {
	if strings.TrimSpace(baseImage) == "" {
		baseImage = DefaultBaseImage
	}
	normalized, err := NormalizeSelections(selections)
	if err != nil {
		return "", "", err
	}
	if len(normalized) == 0 {
		return baseImage, baseImage, nil
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return "", "", err
	}
	h := sha256.New()
	_, _ = h.Write([]byte(baseImage))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(payload)
	tag := "local/aktion-runner:" + hex.EncodeToString(h.Sum(nil))[:16]
	return tag, baseImage, nil
}
