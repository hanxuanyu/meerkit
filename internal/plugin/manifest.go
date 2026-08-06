package plugin

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"runtime"
	"strings"

	"meerkit/internal/core"
)

type ProtocolRange struct {
	Min int `json:"min" yaml:"min"`
	Max int `json:"max" yaml:"max"`
}
type Artifact struct {
	GOOS   string `json:"goos" yaml:"goos"`
	GOARCH string `json:"goarch" yaml:"goarch"`
	Path   string `json:"path" yaml:"path"`
	Size   int64  `json:"size" yaml:"size"`
	SHA256 string `json:"sha256" yaml:"sha256"`
}
type Manifest struct {
	SchemaVersion int                 `json:"schema_version" yaml:"schema_version"`
	ID            string              `json:"id" yaml:"id"`
	Name          string              `json:"name" yaml:"name"`
	Version       string              `json:"version" yaml:"version"`
	Vendor        string              `json:"vendor" yaml:"vendor"`
	Description   string              `json:"desp" yaml:"desp"`
	URL           string              `json:"url" yaml:"url"`
	Protocol      ProtocolRange       `json:"protocol" yaml:"protocol"`
	Modules       []core.PluginModule `json:"modules" yaml:"modules"`
	Artifacts     []Artifact          `json:"artifacts" yaml:"artifacts"`
}

var identityPattern = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*$`)
var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
var artifactTargetPattern = regexp.MustCompile(`^[a-z0-9]+$`)
var artifactHashPattern = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

func (m Manifest) Validate(protocolVersion int) error {
	if m.SchemaVersion != 1 {
		return fmt.Errorf("unsupported manifest schema version %d", m.SchemaVersion)
	}
	if !identityPattern.MatchString(m.ID) {
		return fmt.Errorf("invalid plugin id %q", m.ID)
	}
	if strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Vendor) == "" || strings.TrimSpace(m.Description) == "" {
		return fmt.Errorf("plugin name, vendor, and desp are required")
	}
	sourceURL, err := url.ParseRequestURI(m.URL)
	if err != nil || (sourceURL.Scheme != "https" && sourceURL.Scheme != "http") || sourceURL.Host == "" {
		return fmt.Errorf("plugin url must be an absolute HTTP or HTTPS URL")
	}
	if !versionPattern.MatchString(m.Version) {
		return fmt.Errorf("plugin version %q must use semantic versioning", m.Version)
	}
	if m.Protocol.Min < 1 || m.Protocol.Max < m.Protocol.Min {
		return fmt.Errorf("plugin protocol range %d-%d is invalid", m.Protocol.Min, m.Protocol.Max)
	}
	if protocolVersion < m.Protocol.Min || protocolVersion > m.Protocol.Max {
		return fmt.Errorf("plugin protocol range %d-%d does not include host protocol %d", m.Protocol.Min, m.Protocol.Max, protocolVersion)
	}
	if len(m.Modules) == 0 {
		return fmt.Errorf("plugin must declare at least one monitor module")
	}
	seenModules := map[string]struct{}{}
	for _, module := range m.Modules {
		if !identityPattern.MatchString(module.Type) {
			return fmt.Errorf("invalid module type %q", module.Type)
		}
		if _, exists := seenModules[module.Type]; exists {
			return fmt.Errorf("duplicate module type %q", module.Type)
		}
		seenModules[module.Type] = struct{}{}
		if strings.TrimSpace(module.Name) == "" || module.Version == "" || module.ConfigVersion == "" || module.ResultSchemaVersion == "" {
			return fmt.Errorf("module %s name and versions are required", module.Type)
		}
	}
	seenTargets := map[string]struct{}{}
	seenPaths := map[string]struct{}{}
	for _, artifact := range m.Artifacts {
		if err := artifact.Validate(); err != nil {
			return err
		}
		target := artifact.GOOS + "/" + artifact.GOARCH
		if _, exists := seenTargets[target]; exists {
			return fmt.Errorf("duplicate plugin artifact target %s", target)
		}
		seenTargets[target] = struct{}{}
		if _, exists := seenPaths[artifact.Path]; exists {
			return fmt.Errorf("duplicate plugin artifact path %s", artifact.Path)
		}
		seenPaths[artifact.Path] = struct{}{}
	}
	return nil
}

func (a Artifact) Validate() error {
	if !artifactTargetPattern.MatchString(a.GOOS) || !artifactTargetPattern.MatchString(a.GOARCH) {
		return fmt.Errorf("plugin artifact target %q/%q is invalid", a.GOOS, a.GOARCH)
	}
	clean := path.Clean(strings.ReplaceAll(a.Path, "\\", "/"))
	if clean == "." || clean != a.Path || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return fmt.Errorf("invalid artifact path %q", a.Path)
	}
	if a.Size < 1 || !artifactHashPattern.MatchString(a.SHA256) {
		return fmt.Errorf("artifact %s has invalid size or SHA-256", a.Path)
	}
	return nil
}

func (m Manifest) CurrentArtifact() (Artifact, error) {
	var matches []Artifact
	for _, artifact := range m.Artifacts {
		if artifact.GOOS == runtime.GOOS && artifact.GOARCH == runtime.GOARCH {
			matches = append(matches, artifact)
		}
	}
	if len(matches) != 1 {
		return Artifact{}, fmt.Errorf("plugin contains %d artifacts for %s/%s; exactly one is required", len(matches), runtime.GOOS, runtime.GOARCH)
	}
	artifact := matches[0]
	if err := artifact.Validate(); err != nil {
		return Artifact{}, err
	}
	return artifact, nil
}
