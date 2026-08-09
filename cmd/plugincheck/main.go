package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"go.yaml.in/yaml/v3"
	"meerkit/internal/plugin"
	"meerkit/internal/pluginconformance"
)

func main() {
	manifestPath := flag.String("manifest", "", "path to meerkit-plugin.yaml")
	artifactPath := flag.String("artifact", "", "path to the plugin artifact to execute")
	suitePath := flag.String("suite", "", "optional JSON conformance suite")
	timeout := flag.Duration("timeout", 10*time.Second, "timeout for startup and each plugin call")
	flag.Parse()
	if *manifestPath == "" || *artifactPath == "" {
		fail("--manifest and --artifact are required")
	}
	manifestData, err := os.ReadFile(*manifestPath)
	if err != nil {
		fail(err.Error())
	}
	var manifest plugin.Manifest
	if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
		fail(fmt.Sprintf("decode manifest: %v", err))
	}
	var suite pluginconformance.Suite
	if *suitePath != "" {
		suite, err = pluginconformance.LoadSuite(*suitePath)
		if err != nil {
			fail(err.Error())
		}
	}
	report, err := pluginconformance.Check(context.Background(), pluginconformance.Options{
		Manifest:  manifest,
		Artifact:  *artifactPath,
		Suite:     suite,
		Timeout:   *timeout,
		LogOutput: os.Stderr,
	})
	if err != nil {
		fail(err.Error())
	}
	for _, check := range report.Checks {
		fmt.Println("PASS", check)
	}
	fmt.Printf("plugin conformance passed (%d checks)\n", len(report.Checks))
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "plugincheck:", message)
	os.Exit(1)
}
