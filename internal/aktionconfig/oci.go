package aktionconfig

import (
	"context"
	"fmt"

	"github.com/zyf722/aktion/internal/ociregistry"
)

const (
	PresetArtifactType   = "application/vnd.aktion.preset.v1"
	PresetLayerMediaType = "application/vnd.aktion.preset.v1+yaml"
)

type OCIPresetSource struct {
	Registry *ociregistry.Client
}

func NewOCIPresetSource(cacheDir string) *OCIPresetSource {
	return &OCIPresetSource{Registry: ociregistry.NewClient(cacheDir)}
}

func (s *OCIPresetSource) FetchPreset(ctx context.Context, input string) (Document, string, error) {
	if s == nil || s.Registry == nil {
		return Document{}, "", fmt.Errorf("OCI preset resolver is unavailable")
	}
	artifact, err := s.Registry.FetchManifest(ctx, input, ociregistry.ManifestAccept)
	if err != nil {
		return Document{}, "", err
	}
	if artifact.Manifest.ArtifactType != PresetArtifactType {
		return Document{}, "", fmt.Errorf("%s is not an Aktion preset (artifact type %q)", artifact.Reference.String(), artifact.Manifest.ArtifactType)
	}
	var digest string
	for _, layer := range artifact.Manifest.Layers {
		if layer.MediaType == PresetLayerMediaType {
			if digest != "" {
				return Document{}, "", fmt.Errorf("%s contains multiple %s layers", artifact.Reference.String(), PresetLayerMediaType)
			}
			digest = layer.Digest
		}
	}
	if digest == "" {
		return Document{}, "", fmt.Errorf("%s has no %s layer", artifact.Reference.String(), PresetLayerMediaType)
	}
	data, err := s.Registry.FetchBlob(ctx, artifact.Reference, digest)
	if err != nil {
		return Document{}, "", err
	}
	doc, err := Parse(data)
	if err != nil {
		return Document{}, "", fmt.Errorf("parse preset %s: %w", artifact.Reference.String(), err)
	}
	canonical := artifact.Reference.Identity() + "@" + artifact.ManifestDigest
	return doc, canonical, nil
}
