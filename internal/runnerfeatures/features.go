package runnerfeatures

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed features/*/feature.json features/*/install.sh
var files embed.FS

type Manifest struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	DependsOn     []string `json:"dependsOn"`
	InstallsAfter []string `json:"installsAfter"`
}

type Feature struct {
	Manifest Manifest
	Install  []byte
	Digest   string
}

type Plan struct {
	BaseImage  string
	Features   []Feature
	Image      string
	Dockerfile string
}

var Default = []string{"build", "network", "python", "archives", "native"}

func Catalog() (map[string]Feature, error) {
	entries, err := fs.ReadDir(files, "features")
	if err != nil {
		return nil, err
	}
	out := make(map[string]Feature, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		meta, err := files.ReadFile("features/" + id + "/feature.json")
		if err != nil {
			return nil, err
		}
		install, err := files.ReadFile("features/" + id + "/install.sh")
		if err != nil {
			return nil, err
		}
		var manifest Manifest
		if err := json.Unmarshal(meta, &manifest); err != nil {
			return nil, fmt.Errorf("feature %s: %w", id, err)
		}
		if manifest.ID != id || manifest.ID == "" {
			return nil, fmt.Errorf("feature directory %q has id %q", id, manifest.ID)
		}
		h := sha256.New()
		_, _ = h.Write(meta)
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(install)
		out[id] = Feature{Manifest: manifest, Install: install, Digest: hex.EncodeToString(h.Sum(nil))}
	}
	return out, nil
}

func Resolve(requested []string) ([]Feature, error) {
	catalog, err := Catalog()
	if err != nil {
		return nil, err
	}
	selected := map[string]bool{}
	orderHint := map[string]int{}
	var add func(string) error
	add = func(id string) error {
		id = strings.TrimSpace(id)
		f, ok := catalog[id]
		if !ok {
			return fmt.Errorf("unknown runner feature %q", id)
		}
		if selected[id] {
			return nil
		}
		selected[id] = true
		if _, ok := orderHint[id]; !ok {
			orderHint[id] = len(orderHint)
		}
		for _, dep := range f.Manifest.DependsOn {
			if err := add(dep); err != nil {
				return fmt.Errorf("feature %s depends on %s: %w", id, dep, err)
			}
		}
		return nil
	}
	for _, id := range requested {
		if err := add(id); err != nil {
			return nil, err
		}
	}

	indegree := map[string]int{}
	edges := map[string]map[string]bool{}
	addEdge := func(from, to string) {
		if from == to || !selected[from] || !selected[to] {
			return
		}
		if edges[from] == nil {
			edges[from] = map[string]bool{}
		}
		if !edges[from][to] {
			edges[from][to] = true
			indegree[to]++
		}
	}
	for id := range selected {
		indegree[id] += 0
		f := catalog[id]
		for _, dep := range f.Manifest.DependsOn {
			addEdge(dep, id)
		}
		for _, after := range f.Manifest.InstallsAfter {
			addEdge(after, id)
		}
	}

	var result []Feature
	for len(result) < len(selected) {
		ready := make([]string, 0)
		for id := range selected {
			if indegree[id] == 0 {
				already := false
				for _, f := range result {
					if f.Manifest.ID == id {
						already = true
						break
					}
				}
				if !already {
					ready = append(ready, id)
				}
			}
		}
		if len(ready) == 0 {
			return nil, fmt.Errorf("runner feature dependency cycle detected")
		}
		sort.SliceStable(ready, func(i, j int) bool {
			ai, aok := orderHint[ready[i]]
			bi, bok := orderHint[ready[j]]
			if aok != bok {
				return aok
			}
			if aok && ai != bi {
				return ai < bi
			}
			return ready[i] < ready[j]
		})
		id := ready[0]
		result = append(result, catalog[id])
		for to := range edges[id] {
			indegree[to]--
		}
		indegree[id] = -1
	}
	return result, nil
}

func BuildPlan(baseImage string, requested []string) (Plan, error) {
	if strings.TrimSpace(baseImage) == "" {
		baseImage = "ghcr.io/actions/actions-runner:latest"
	}
	resolved, err := Resolve(requested)
	if err != nil {
		return Plan{}, err
	}
	if len(resolved) == 0 {
		return Plan{BaseImage: baseImage, Features: nil, Image: baseImage}, nil
	}
	h := sha256.New()
	_, _ = h.Write([]byte(baseImage))
	for _, f := range resolved {
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(f.Manifest.ID))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(f.Digest))
	}
	tag := "local/aktion-runner:" + hex.EncodeToString(h.Sum(nil))[:16]

	var b strings.Builder
	fmt.Fprintf(&b, "FROM %s AS aktion-base\n", baseImage)
	b.WriteString("USER root\n")
	previous := "aktion-base"
	for i, f := range resolved {
		stage := fmt.Sprintf("aktion-feature-%02d-%s", i+1, f.Manifest.ID)
		fmt.Fprintf(&b, "\nFROM %s AS %s\n", previous, stage)
		fmt.Fprintf(&b, "COPY features/%s/install.sh /tmp/aktion-feature-install.sh\n", f.Manifest.ID)
		b.WriteString("RUN chmod +x /tmp/aktion-feature-install.sh \\\n    && /tmp/aktion-feature-install.sh \\\n    && rm -f /tmp/aktion-feature-install.sh\n")
		previous = stage
	}
	fmt.Fprintf(&b, "\nFROM %s AS final\n", previous)
	b.WriteString("USER runner\n")
	return Plan{BaseImage: baseImage, Features: resolved, Image: tag, Dockerfile: b.String()}, nil
}

func InstallScript(id string) ([]byte, error) {
	return files.ReadFile("features/" + id + "/install.sh")
}
