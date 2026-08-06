package main

import (
	"embed"
	"io/fs"
	"os"

	"meerkit/internal/command"
)

//go:embed web/dist/*
var embeddedFrontend embed.FS
var version = "dev"

func main() {
	frontend, err := fs.Sub(embeddedFrontend, "web/dist")
	if err != nil {
		os.Exit(1)
	}
	os.Exit(command.Execute(command.Dependencies{Frontend: frontend, Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr, Version: version}, os.Args[1:]))
}
