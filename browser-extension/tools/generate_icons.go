package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	sourcePath := filepath.Join("web", "MeerKit.png")
	targetPath := filepath.Join("browser-extension", "icons", "meerkit.png")

	contents, err := os.ReadFile(sourcePath)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(targetPath, contents, 0o644); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("copied %s to %s without resizing or overlays\n", sourcePath, targetPath)
}
