package ociregistry

import (
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

const ManifestAccept = "application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json"

var (
	pathPattern   = regexp.MustCompile("^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)+$")
	tagPattern    = regexp.MustCompile("^[a-z0-9_][a-z0-9._-]{0,127}$")
	bearerParamRE = regexp.MustCompile("([a-zA-Z][a-zA-Z0-9_-]*)=\\\"([^\\\"]*)\\\"")
)

type Reference struct {
	Registry   string
	Repository string
	Reference  string
	Digest     bool
}

func ParseReference(input string) (Reference, error) {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" || strings.Contains(input, "://") {
		return Reference{}, fmt.Errorf("invalid OCI reference %q", input)
	}
	var resource, reference string
	isDigest := false
	if at := strings.LastIndex(input, "@"); at >= 0 {
		resource = input[:at]
		reference = input[at+1:]
		isDigest = true
		if !strings.HasPrefix(reference, "sha256:") || len(strings.TrimPrefix(reference, "sha256:")) != 64 {
			return Reference{}, fmt.Errorf("OCI digest must be sha256:<64 hex characters>: %q", input)
		}
		if _, err := hex.DecodeString(strings.TrimPrefix(reference, "sha256:")); err != nil {
			return Reference{}, fmt.Errorf("invalid OCI digest %q", reference)
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
		if !tagPattern.MatchString(reference) {
			return Reference{}, fmt.Errorf("invalid OCI tag %q", reference)
		}
	}
	slash := strings.Index(resource, "/")
	if slash <= 0 || slash == len(resource)-1 {
		return Reference{}, fmt.Errorf("OCI reference must look like registry/namespace/name[:tag]: %q", input)
	}
	registry := resource[:slash]
	repository := resource[slash+1:]
	if !pathPattern.MatchString(repository) {
		return Reference{}, fmt.Errorf("invalid OCI repository path %q", repository)
	}
	host := registry
	if colon := strings.LastIndex(host, ":"); colon > 0 {
		host = host[:colon]
	}
	if !strings.Contains(host, ".") && !strings.HasPrefix(host, "localhost") {
		return Reference{}, fmt.Errorf("OCI registry %q must be a hostname", registry)
	}
	return Reference{Registry: registry, Repository: repository, Reference: reference, Digest: isDigest}, nil
}

func (r Reference) Identity() string { return r.Registry + "/" + r.Repository }

func (r Reference) String() string {
	if r.Digest {
		return r.Identity() + "@" + r.Reference
	}
	return r.Identity() + ":" + r.Reference
}

func (r Reference) Name() string {
	_, name := path.Split(r.Repository)
	return name
}

type Descriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

type Manifest struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	ArtifactType  string       `json:"artifactType,omitempty"`
	Config        Descriptor   `json:"config"`
	Layers        []Descriptor `json:"layers"`
}

type Artifact struct {
	Reference      Reference
	Manifest       Manifest
	ManifestDigest string
}

type Client struct {
	HTTP     *http.Client
	CacheDir string

	mu     sync.Mutex
	tokens map[string]string
}

func DefaultCacheDir() string {
	if dir, err := os.UserCacheDir(); err == nil && dir != "" {
		return filepath.Join(dir, "aktion", "oci")
	}
	return filepath.Join(os.TempDir(), "aktion-oci-cache")
}

func NewClient(cacheDir string) *Client {
	if cacheDir == "" {
		cacheDir = DefaultCacheDir()
	}
	return &Client{
		HTTP:     &http.Client{Timeout: 2 * time.Minute},
		CacheDir: cacheDir,
		tokens:   map[string]string{},
	}
}

func (c *Client) FetchManifest(ctx context.Context, input, accept string) (Artifact, error) {
	ref, err := ParseReference(input)
	if err != nil {
		return Artifact{}, err
	}
	if accept == "" {
		accept = ManifestAccept
	}
	endpoint := fmt.Sprintf("https://%s/v2/%s/manifests/%s", ref.Registry, ref.Repository, ref.Reference)
	data, headers, err := c.get(ctx, endpoint, accept, ref)
	if err != nil {
		return Artifact{}, fmt.Errorf("fetch OCI manifest %s: %w", ref.String(), err)
	}
	digest := DigestBytes(data)
	if ref.Digest && digest != ref.Reference {
		return Artifact{}, fmt.Errorf("OCI manifest digest mismatch for %s: got %s", ref.String(), digest)
	}
	if advertised := headers.Get("Docker-Content-Digest"); advertised != "" && advertised != digest {
		return Artifact{}, fmt.Errorf("OCI registry digest mismatch for %s: header=%s computed=%s", ref.String(), advertised, digest)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Artifact{}, fmt.Errorf("parse OCI manifest %s: %w", ref.String(), err)
	}
	if manifest.SchemaVersion != 2 {
		return Artifact{}, fmt.Errorf("%s has unsupported OCI schemaVersion %d", ref.String(), manifest.SchemaVersion)
	}
	return Artifact{
		Reference:      ref,
		Manifest:       manifest,
		ManifestDigest: digest,
	}, nil
}

func (c *Client) FetchBlob(ctx context.Context, ref Reference, digest string) ([]byte, error) {
	if !strings.HasPrefix(digest, "sha256:") || len(strings.TrimPrefix(digest, "sha256:")) != 64 {
		return nil, fmt.Errorf("invalid OCI blob digest %q", digest)
	}
	cachePath := filepath.Join(c.CacheDir, "blobs", "sha256", strings.TrimPrefix(digest, "sha256:"))
	if data, err := os.ReadFile(cachePath); err == nil {
		if DigestBytes(data) == digest {
			return data, nil
		}
		_ = os.Remove(cachePath)
	}

	endpoint := fmt.Sprintf("https://%s/v2/%s/blobs/%s", ref.Registry, ref.Repository, digest)
	data, _, err := c.get(ctx, endpoint, "application/octet-stream", ref)
	if err != nil {
		return nil, fmt.Errorf("fetch OCI blob %s: %w", digest, err)
	}
	if got := DigestBytes(data); got != digest {
		return nil, fmt.Errorf("OCI blob digest mismatch: want %s got %s", digest, got)
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

func (c *Client) get(ctx context.Context, endpoint, accept string, ref Reference) ([]byte, http.Header, error) {
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
	return body, resp.Header, err
}

func (c *Client) bearerToken(ctx context.Context, challenge string, ref Reference) (string, error) {
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

func DigestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
