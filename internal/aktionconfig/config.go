package aktionconfig

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/zyf722/aktion/internal/runnerfeatures"
	"go.yaml.in/yaml/v3"
)

const SchemaVersion = 1

type Document struct {
	Version int      `yaml:"version" json:"version"`
	Extends []string `yaml:"extends,omitempty" json:"extends,omitempty"`
	Runner  Runner   `yaml:"runner,omitempty" json:"runner,omitempty"`
}

type Runner struct {
	Image    string         `yaml:"image,omitempty" json:"image,omitempty"`
	Features map[string]any `yaml:"features,omitempty" json:"features,omitempty"`
}

type Resolved struct {
	Image    string                    `json:"image,omitempty"`
	Features runnerfeatures.Selections `json:"features"`
	Presets  []string                  `json:"presets,omitempty"`
}

type PresetSource interface {
	FetchPreset(context.Context, string) (Document, string, error)
}

type Resolver struct {
	Presets PresetSource
}

func Parse(data []byte) (Document, error) {
	var doc Document
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("invalid Aktion YAML: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return Document{}, fmt.Errorf("Aktion YAML must contain exactly one document")
		}
		return Document{}, fmt.Errorf("invalid Aktion YAML: %w", err)
	}
	if doc.Version != SchemaVersion {
		return Document{}, fmt.Errorf("unsupported Aktion config version %d (want %d)", doc.Version, SchemaVersion)
	}
	for i, ref := range doc.Extends {
		doc.Extends[i] = strings.TrimSpace(ref)
		if doc.Extends[i] == "" {
			return Document{}, fmt.Errorf("extends[%d] cannot be empty", i)
		}
	}
	return doc, nil
}

func (r *Resolver) Resolve(ctx context.Context, doc Document) (Resolved, error) {
	if doc.Version != SchemaVersion {
		return Resolved{}, fmt.Errorf("unsupported Aktion config version %d (want %d)", doc.Version, SchemaVersion)
	}
	return r.apply(ctx, Resolved{Features: runnerfeatures.Selections{}}, doc, map[string]bool{}, 0)
}

func (r *Resolver) apply(ctx context.Context, base Resolved, doc Document, stack map[string]bool, depth int) (Resolved, error) {
	if depth > 32 {
		return Resolved{}, fmt.Errorf("Aktion preset inheritance exceeds 32 levels")
	}
	out := cloneResolved(base)
	for _, ref := range doc.Extends {
		if r.Presets == nil {
			return Resolved{}, fmt.Errorf("cannot resolve preset %s: OCI preset resolver is unavailable", ref)
		}
		preset, canonical, err := r.Presets.FetchPreset(ctx, ref)
		if err != nil {
			return Resolved{}, fmt.Errorf("resolve preset %s: %w", ref, err)
		}
		if stack[canonical] {
			return Resolved{}, fmt.Errorf("Aktion preset inheritance cycle detected at %s", canonical)
		}
		nextStack := make(map[string]bool, len(stack)+1)
		for key, value := range stack {
			nextStack[key] = value
		}
		nextStack[canonical] = true
		out, err = r.apply(ctx, out, preset, nextStack, depth+1)
		if err != nil {
			return Resolved{}, err
		}
		out.Presets = append(out.Presets, canonical)
	}
	if doc.Runner.Image != "" {
		out.Image = strings.TrimSpace(doc.Runner.Image)
	}
	features, err := ApplyFeaturePatch(out.Features, doc.Runner.Features)
	if err != nil {
		return Resolved{}, err
	}
	out.Features = features
	return out, nil
}

func ApplyFeaturePatch(base runnerfeatures.Selections, patch map[string]any) (runnerfeatures.Selections, error) {
	normalized, err := runnerfeatures.NormalizeSelections(base)
	if err != nil {
		return nil, err
	}
	out := runnerfeatures.CloneSelections(normalized)
	if out == nil {
		out = runnerfeatures.Selections{}
	}
	refs := make([]string, 0, len(patch))
	for ref := range patch {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	for _, rawRef := range refs {
		ref, err := runnerfeatures.NormalizeReference(rawRef)
		if err != nil {
			return nil, fmt.Errorf("runner.features %q: %w", rawRef, err)
		}
		value := patch[rawRef]
		switch v := value.(type) {
		case bool:
			if !v {
				delete(out, ref)
				continue
			}
			if _, ok := out[ref]; !ok {
				out[ref] = runnerfeatures.Options{}
			}
		case string:
			opts := cloneOptions(out[ref])
			opts["version"] = v
			out[ref] = opts
		case map[string]any:
			opts := cloneOptions(out[ref])
			for key, option := range v {
				switch option.(type) {
				case string, bool:
					opts[key] = option
				default:
					return nil, fmt.Errorf("runner.features %q option %q must be a string or boolean", rawRef, key)
				}
			}
			out[ref] = opts
		case nil:
			return nil, fmt.Errorf("runner.features %q cannot be null", rawRef)
		default:
			return nil, fmt.Errorf("runner.features %q must be an object, string, or boolean", rawRef)
		}
	}
	return runnerfeatures.NormalizeSelections(out)
}

func MergeSelections(base, overlay runnerfeatures.Selections) (runnerfeatures.Selections, error) {
	patch := make(map[string]any, len(overlay))
	for ref, options := range overlay {
		copied := make(map[string]any, len(options))
		for key, value := range options {
			copied[key] = value
		}
		patch[ref] = copied
	}
	return ApplyFeaturePatch(base, patch)
}

func cloneResolved(in Resolved) Resolved {
	return Resolved{
		Image:    in.Image,
		Features: runnerfeatures.CloneSelections(in.Features),
		Presets:  append([]string(nil), in.Presets...),
	}
}

func cloneOptions(in runnerfeatures.Options) runnerfeatures.Options {
	out := make(runnerfeatures.Options, len(in)+1)
	for key, value := range in {
		out[key] = value
	}
	return out
}
