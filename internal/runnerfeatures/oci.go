package runnerfeatures

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/zyf722/aktion/internal/ociregistry"
)

const (
	devcontainerConfigMediaType = "application/vnd.devcontainers"
	devcontainerLayerMediaType  = "application/vnd.devcontainers.layer.v1+tar"
)

type OCIReference = ociregistry.Reference

func ParseOCIReference(input string) (OCIReference, error) {
	return ociregistry.ParseReference(input)
}

type OCIClient struct {
	Registry *ociregistry.Client
}

func DefaultCacheDir() string { return ociregistry.DefaultCacheDir() }

func NewOCIClient(cacheDir string) *OCIClient {
	return &OCIClient{Registry: ociregistry.NewClient(cacheDir)}
}

func (c *OCIClient) FetchFeature(ctx context.Context, input string) (FeaturePackage, error) {
	if c == nil || c.Registry == nil {
		return FeaturePackage{}, fmt.Errorf("OCI resolver is unavailable")
	}
	artifact, err := c.Registry.FetchManifest(ctx, input, ociregistry.ManifestAccept)
	if err != nil {
		return FeaturePackage{}, err
	}
	ref := artifact.Reference
	manifest := artifact.Manifest
	if manifest.Config.MediaType != devcontainerConfigMediaType {
		return FeaturePackage{}, fmt.Errorf("%s is not a Dev Container Feature artifact (config media type %q)", ref.String(), manifest.Config.MediaType)
	}

	var layerDigest string
	for _, layer := range manifest.Layers {
		if layer.MediaType == devcontainerLayerMediaType {
			layerDigest = layer.Digest
			break
		}
	}
	if layerDigest == "" {
		return FeaturePackage{}, fmt.Errorf("%s has no %s layer", ref.String(), devcontainerLayerMediaType)
	}

	archive, err := c.Registry.FetchBlob(ctx, ref, layerDigest)
	if err != nil {
		return FeaturePackage{}, err
	}
	files, err := extractFeatureArchive(archive)
	if err != nil {
		return FeaturePackage{}, fmt.Errorf("extract %s: %w", ref.String(), err)
	}

	var metadataBytes []byte
	hasInstall := false
	for _, f := range files {
		switch f.Path {
		case "devcontainer-feature.json":
			metadataBytes = f.Data
		case "install.sh":
			hasInstall = true
		}
	}
	if len(metadataBytes) == 0 || !hasInstall {
		return FeaturePackage{}, fmt.Errorf("%s must contain devcontainer-feature.json and install.sh", ref.String())
	}

	var metadata Manifest
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return FeaturePackage{}, fmt.Errorf("parse devcontainer-feature.json for %s: %w", ref.String(), err)
	}
	if metadata.ID == "" || metadata.Version == "" || metadata.Name == "" {
		return FeaturePackage{}, fmt.Errorf("%s metadata must contain id, version, and name", ref.String())
	}
	if !strings.EqualFold(metadata.ID, ref.Name()) {
		return FeaturePackage{}, fmt.Errorf("%s metadata id %q does not match artifact name %q", ref.String(), metadata.ID, ref.Name())
	}

	return FeaturePackage{
		Reference:      ref.String(),
		Identity:       ref.Identity(),
		Canonical:      ref.Identity() + "@" + artifact.ManifestDigest,
		ManifestDigest: artifact.ManifestDigest,
		Manifest:       metadata,
		Files:          files,
	}, nil
}

func extractFeatureArchive(data []byte) ([]FeatureFile, error) {
	var reader io.Reader = bytes.NewReader(data)
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader = gz
	}

	tr := tar.NewReader(reader)
	var files []FeatureFile
	var total int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if path.IsAbs(hdr.Name) {
			return nil, fmt.Errorf("unsafe archive path %q", hdr.Name)
		}
		name := path.Clean(hdr.Name)
		if name == "" || name == "." {
			continue
		}
		if name == ".." || strings.HasPrefix(name, "../") {
			return nil, fmt.Errorf("unsafe archive path %q", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg, tar.TypeRegA:
			if hdr.Size < 0 || hdr.Size > 32<<20 {
				return nil, fmt.Errorf("feature file %q has unreasonable size %d", name, hdr.Size)
			}
			total += hdr.Size
			if total > 128<<20 {
				return nil, fmt.Errorf("feature archive exceeds 128 MiB")
			}
			fileData, err := io.ReadAll(io.LimitReader(tr, hdr.Size))
			if err != nil {
				return nil, err
			}
			files = append(files, FeatureFile{Path: name, Mode: os.FileMode(hdr.Mode).Perm(), Data: fileData})
		default:
			return nil, fmt.Errorf("unsupported archive entry type for %q", name)
		}
	}
	return files, nil
}
