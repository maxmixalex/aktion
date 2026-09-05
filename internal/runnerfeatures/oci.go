package runnerfeatures

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	devcontainerConfigMediaType = "application/vnd.devcontainers"
	devcontainerLayerMediaType  = "application/vnd.devcontainers.layer.v1+tar"
)

var (
	ociPathPattern = regexp.MustCompile("^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)+$")
	ociTagPattern  = regexp.MustCompile("^[a-z0-9_][a-z0-9._-]{0,127}$")
	bearerParamRE  = regexp.MustCompile("([a-zA-Z][a-zA-Z0-9_-]*)=\\\"([^\\\"]*)\\\"")
)

type OCIReference struct {
	Registry   string
	Repository string
	Reference  string
	Digest     bool
}

func ParseOCIReference(input string) (OCIReference, error) {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" || strings.Contains(input, "://") {
		return OCIReference{}, fmt.Errorf("invalid OCI feature reference %q", input)
	}

	var resource, reference string
	isDigest := false
	if at := strings.LastIndex(input, "@"); at >= 0 {
		resource = input[:at]
		reference = input[at+1:]
		isDigest = true
		if !strings.HasPrefix(reference, "sha256:") || len(strings.TrimPrefix(reference, "sha256:")) != 64 {
			return OCIReference{}, fmt.Errorf("OCI feature digest must be sha256:<64 hex characters>: %q", input)
		}
		if _, err := hex.DecodeString(strings.TrimPrefix(reference, "sha256:")); err != nil {
			return OCIReference{}, fmt.Errorf("invalid OCI feature digest %q", reference)
		}
	} else {
		lastSlash := strings.LastIndex(input, "/")
		lastColon := strings.LastIndex(input, ":")
		if lastColon > lastSlash {
			resource = input[:lastColon]
			reference = input[lastColon+1:]
		} else {
			resource = input
			reference = "latest"
		}
		if !ociTagPattern.MatchString(reference) {
			return OCIReference{}, fmt.Errorf("invalid OCI feature tag %q", reference)
		}
	}

	slash := strings.Index(resource, "/")
	if slash <= 0 || slash == len(resource)-1 {
		return OCIReference{}, fmt.Errorf("OCI feature reference must look like registry/namespace/feature[:version]: %q", input)
	}
	registry := resource[:slash]
	repository := resource[slash+1:]
	if !ociPathPattern.MatchString(repository) {
		return OCIReference{}, fmt.Errorf("invalid OCI repository path %q", repository)
	}
	if !strings.Contains(registry, ".") && !strings.HasPrefix(registry, "localhost") {
		return OCIReference{}, fmt.Errorf("OCI feature registry %q must be a hostname", registry)
	}
	return OCIReference{Registry: registry, Repository: repository, Reference: reference, Digest: isDigest}, nil
}

func (r OCIReference) Identity() string {
	return r.Registry + "/" + r.Repository
}

func (r OCIReference) String() string {
	if r.Digest {
		return r.Identity() + "@" + r.Reference
	}
	return r.Identity() + ":" + r.Reference
}

func (r OCIReference) FeatureID() string {
	_, id := path.Split(r.Repository)
	return id
}

type ociManifest struct {
	SchemaVersion int
	MediaType     string
	Config        struct {
		MediaType string
		Digest    string
		Size      int64
	}
	Layers []struct {
		MediaType string
		Digest    string
		Size      int64
	}
}

type OCIClient struct {
	HTTP     *http.Client
	CacheDir string

	mu     sync.Mutex
	tokens map[string]string
}

func DefaultCacheDir() string {
	if dir, err := os.UserCacheDir(); err == nil && dir != "" {
		return filepath.Join(dir, "aktion", "features")
	}
	return filepath.Join(os.TempDir(), "aktion-feature-cache")
}

func NewOCIClient(cacheDir string) *OCIClient {
	if cacheDir == "" {
		cacheDir = DefaultCacheDir()
	}
	return &OCIClient{
		HTTP:     &http.Client{Timeout: 2 * time.Minute},
		CacheDir: cacheDir,
		tokens:   map[string]string{},
	}
}

func (c *OCIClient) FetchFeature(ctx context.Context, input string) (FeaturePackage, error) {
	ref, err := ParseOCIReference(input)
	if err != nil {
		return FeaturePackage{}, err
	}
	manifestURL := fmt.Sprintf("https://%s/v2/%s/manifests/%s", ref.Registry, ref.Repository, ref.Reference)
	manifestBytes, headers, err := c.get(ctx, manifestURL, "application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json", ref)
	if err != nil {
		return FeaturePackage{}, fmt.Errorf("fetch OCI manifest %s: %w", ref.String(), err)
	}
	manifestDigest := digestBytes(manifestBytes)
	if ref.Digest && manifestDigest != ref.Reference {
		return FeaturePackage{}, fmt.Errorf("OCI manifest digest mismatch for %s: got %s", ref.String(), manifestDigest)
	}
	if advertised := headers.Get("Docker-Content-Digest"); advertised != "" && advertised != manifestDigest {
		return FeaturePackage{}, fmt.Errorf("OCI registry digest mismatch for %s: header=%s computed=%s", ref.String(), advertised, manifestDigest)
	}

	var manifest ociManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return FeaturePackage{}, fmt.Errorf("parse OCI manifest %s: %w", ref.String(), err)
	}
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

	archive, err := c.fetchBlob(ctx, ref, layerDigest)
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
	if !strings.EqualFold(metadata.ID, ref.FeatureID()) {
		return FeaturePackage{}, fmt.Errorf("%s metadata id %q does not match artifact name %q", ref.String(), metadata.ID, ref.FeatureID())
	}

	return FeaturePackage{
		Reference:      ref.String(),
		Identity:       ref.Identity(),
		Canonical:      ref.Identity() + "@" + manifestDigest,
		ManifestDigest: manifestDigest,
		Manifest:       metadata,
		Files:          files,
	}, nil
}

func (c *OCIClient) fetchBlob(ctx context.Context, ref OCIReference, digest string) ([]byte, error) {
	if !strings.HasPrefix(digest, "sha256:") || len(strings.TrimPrefix(digest, "sha256:")) != 64 {
		return nil, fmt.Errorf("invalid feature layer digest %q", digest)
	}
	cachePath := filepath.Join(c.CacheDir, "blobs", "sha256", strings.TrimPrefix(digest, "sha256:"))
	if data, err := os.ReadFile(cachePath); err == nil {
		if digestBytes(data) == digest {
			return data, nil
		}
		_ = os.Remove(cachePath)
	}

	blobURL := fmt.Sprintf("https://%s/v2/%s/blobs/%s", ref.Registry, ref.Repository, digest)
	data, _, err := c.get(ctx, blobURL, "application/octet-stream", ref)
	if err != nil {
		return nil, fmt.Errorf("fetch OCI feature layer %s: %w", digest, err)
	}
	if got := digestBytes(data); got != digest {
		return nil, fmt.Errorf("OCI feature layer digest mismatch: want %s got %s", digest, got)
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err == nil {
		tmp := cachePath + ".tmp"
		if err := os.WriteFile(tmp, data, 0644); err == nil {
			_ = os.Rename(tmp, cachePath)
		}
		_ = os.Remove(tmp)
	}
	return data, nil
}

func (c *OCIClient) get(ctx context.Context, endpoint, accept string, ref OCIReference) ([]byte, http.Header, error) {
	do := func(token string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		req.Header.Set("User-Agent", "aktion")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		return c.HTTP.Do(req)
	}

	resp, err := do("")
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		challenge := resp.Header.Get("WWW-Authenticate")
		_ = resp.Body.Close()
		token, err := c.bearerToken(ctx, challenge, ref)
		if err != nil {
			return nil, nil, err
		}
		resp, err = do(token)
		if err != nil {
			return nil, nil, err
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		return nil, resp.Header, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 128<<20))
	if err != nil {
		return nil, resp.Header, err
	}
	return body, resp.Header, nil
}

func (c *OCIClient) bearerToken(ctx context.Context, challenge string, ref OCIReference) (string, error) {
	if !strings.HasPrefix(strings.ToLower(challenge), "bearer ") {
		return "", fmt.Errorf("OCI registry requires unsupported authentication: %q", challenge)
	}
	params := map[string]string{}
	for _, match := range bearerParamRE.FindAllStringSubmatch(challenge, -1) {
		params[strings.ToLower(match[1])] = match[2]
	}
	realm := params["realm"]
	if realm == "" {
		return "", fmt.Errorf("OCI bearer challenge is missing realm")
	}
	u, err := url.Parse(realm)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "", fmt.Errorf("invalid OCI token realm %q", realm)
	}
	q := u.Query()
	if service := params["service"]; service != "" {
		q.Set("service", service)
	}
	scope := params["scope"]
	if scope == "" {
		scope = "repository:" + ref.Repository + ":pull"
	}
	q.Set("scope", scope)
	u.RawQuery = q.Encode()

	cacheKey := u.String()
	c.mu.Lock()
	if token := c.tokens[cacheKey]; token != "" {
		c.mu.Unlock()
		return token, nil
	}
	c.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "aktion")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("OCI token service %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	token, _ := payload["token"].(string)
	if token == "" {
		token, _ = payload["access_token"].(string)
	}
	if token == "" {
		return "", fmt.Errorf("OCI token service returned an empty token")
	}
	c.mu.Lock()
	c.tokens[cacheKey] = token
	c.mu.Unlock()
	return token, nil
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
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
