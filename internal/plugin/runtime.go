package plugin

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"meerkit/internal/core"
)

func NewArtifactCommand(artifactPath string, runtimeConfig ArtifactRuntime) (*exec.Cmd, error) {
	if err := runtimeConfig.Validate(); err != nil {
		return nil, err
	}
	if runtimeConfig.Mode == "direct" {
		return exec.Command(artifactPath, runtimeConfig.Args...), nil
	}
	commandPath, err := exec.LookPath(runtimeConfig.Command)
	if err != nil {
		return nil, fmt.Errorf("plugin runtime command %q is unavailable: %w", runtimeConfig.Command, err)
	}
	args := make([]string, len(runtimeConfig.Args))
	for index, argument := range runtimeConfig.Args {
		if argument == "{artifact}" {
			argument = artifactPath
		}
		args[index] = argument
	}
	return exec.Command(commandPath, args...), nil
}

func commandForInstallation(installation core.PluginInstallation) (*exec.Cmd, error) {
	if len(installation.Manifest) == 0 {
		return nil, fmt.Errorf("installed plugin manifest is required")
	}
	var manifest Manifest
	if err := json.Unmarshal(installation.Manifest, &manifest); err != nil {
		return nil, fmt.Errorf("decode installed plugin manifest: %w", err)
	}
	var artifact *Artifact
	if len(manifest.Artifacts) != 0 {
		current, err := manifest.CurrentArtifact()
		if err != nil {
			return nil, err
		}
		artifact = &current
	}
	runtimeConfig, err := manifest.ResolveRuntime(artifact)
	if err != nil {
		return nil, err
	}
	return NewArtifactCommand(installation.BinaryPath, runtimeConfig)
}
