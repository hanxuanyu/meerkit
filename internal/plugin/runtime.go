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
	runtimeConfig := ArtifactRuntime{Mode: "direct"}
	if len(installation.Manifest) != 0 {
		var manifest Manifest
		if err := json.Unmarshal(installation.Manifest, &manifest); err != nil {
			return nil, fmt.Errorf("decode installed plugin manifest: %w", err)
		}
		if len(manifest.Artifacts) != 0 {
			artifact, err := manifest.CurrentArtifact()
			if err != nil {
				return nil, err
			}
			runtimeConfig = artifact.RuntimeConfig()
		}
	}
	return NewArtifactCommand(installation.BinaryPath, runtimeConfig)
}
