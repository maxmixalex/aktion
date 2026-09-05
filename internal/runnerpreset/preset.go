package runnerpreset

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zyf722/aktion/internal/ociregistry"
	"github.com/zyf722/aktion/internal/runnerfeatures"
)

const (
	Version        = 1
	ArtifactType   = "application/vnd.aktion.runner-preset.v1"
	LayerMediaType = "application/vnd.aktion.runner-preset.v1+json"
)

type Preset struct {
	Version        int                       `json:"version"`
	RunnerImage    string                    `json:"runnerImage,omitempty"`
	RunnerFeatures runnerfeatures.Selections `json:"runnerFeatures,omitempty"`
}

type Resolved struct {
	RunnerImage    string
	RunnerFeatures runnerfeatures.Selections
}

type Source struct {
	Registry *ociregistry.Client
}

func NewSource(cacheDir string) *Source {
	return &Source{Registry: ociregistry.NewClient(cacheDir)}
}

func (s *Source) Fetch(ctx context.Context, input string) (Resolved, error) {
	artifact, err := s.Registry.FetchManifest(ctx, input, ociregistry.ManifestAccept)
	if err != nil {
		return Resolved{}, err
	}
	if artifact.Manifest.ArtifactType != ArtifactType {
		return Resolved{}, fmt.Errorf("%s is not an Aktion runner preset (artifact type %q)", artifact.Reference.String(), artifact.Manifest.ArtifactType)
	}
	var layerDigest string
	for _, layer := range artifact.Manifest.Layers {
		if layer.MediaType != LayerMediaType {
			continue
		}
		if layerDigest != "" {
			return Resolved{}, fmt.Errorf("%s contains multiple %s layers", artifact.Reference.String(), LayerMediaType)
		}
		layerDigest = layer.Digest
	}
	if layerDigest == "" {
		return Resolved{}, fmt.Errorf("%s has no %s layer", artifact.Reference.String(), LayerMediaType)
	}
	data, err := s.Registry.FetchBlob(ctx, artifact.Reference, layerDigest)
	if err != nil {
		return Resolved{}, err
	}
	var preset Preset
	if err := json.Unmarshal(data, &preset); err != nil {
		return Resolved{}, fmt.Errorf("parse runner preset %s: %w", artifact.Reference.String(), err)
	}
	if preset.Version != Version {
		return Resolved{}, fmt.Errorf("runner preset %s has unsupported version %d (want %d)", artifact.Reference.String(), preset.Version, Version)
	}
	features, err := runnerfeatures.NormalizeSelections(preset.RunnerFeatures)
	if err != nil {
		return Resolved{}, fmt.Errorf("runner preset %s runnerFeatures: %w", artifact.Reference.String(), err)
	}
	return Resolved{
		RunnerImage:    strings.TrimSpace(preset.RunnerImage),
		RunnerFeatures: features,
	}, nil
}

func MergeFeatures(base, overlay runnerfeatures.Selections) (runnerfeatures.Selections, error) {
	base, err := runnerfeatures.NormalizeSelections(base)
	if err != nil {
		return nil, err
	}
	out := runnerfeatures.CloneSelections(base)
	if out == nil {
		out = runnerfeatures.Selections{}
	}
	seen := map[string]bool{}
	for rawRef, options := range overlay {
		ref, err := runnerfeatures.NormalizeReference(rawRef)
		if err != nil {
			return nil, err
		}
		if seen[ref] {
			return nil, fmt.Errorf("duplicate feature after normalization: %q", ref)
		}
		seen[ref] = true
		if options == nil {
			delete(out, ref)
			continue
		}
		normalized, err := runnerfeatures.NormalizeSelections(runnerfeatures.Selections{ref: options})
		if err != nil {
			return nil, err
		}
		merged := runnerfeatures.Options{}
		if existing, ok := out[ref]; ok {
			for key, value := range existing {
				merged[key] = value
			}
		}
		for key, value := range normalized[ref] {
			merged[key] = value
		}
		out[ref] = merged
	}
	return out, nil
}
