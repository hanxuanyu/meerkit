package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hanxuanyu/meerkit/sdk"
	"go.yaml.in/yaml/v3"
	pluginpkg "meerkit/internal/plugin"
)

type target struct{ goos, goarch string }

func main() {
	pluginDir := flag.String("plugin", "", "plugin source directory")
	outputDir := flag.String("output", "dist/plugins", "output directory")
	targetsFlag := flag.String("targets", "current", "comma-separated GOOS/GOARCH targets")
	combined := flag.Bool("combined", false, "place all targets in one zip archive")
	privateKeyPath := flag.String("sign-key", "", "base64 Ed25519 private key file")
	keyID := flag.String("key-id", "", "signing key identifier")
	generateKeyPath := flag.String("generate-key", "", "generate Ed25519 key files using this path prefix")
	flag.Parse()
	if *generateKeyPath != "" {
		privatePath, publicPath, err := generateKey(*generateKeyPath)
		if err != nil {
			fail(err.Error())
		}
		fmt.Printf("private key: %s\npublic key: %s\n", privatePath, publicPath)
		return
	}
	if *pluginDir == "" {
		fail("--plugin is required")
	}
	targets, err := parseTargets(*targetsFlag)
	if err != nil {
		fail(err.Error())
	}
	manifestBytes, err := os.ReadFile(filepath.Join(*pluginDir, "meerkit-plugin.yaml"))
	if err != nil {
		fail(err.Error())
	}
	var manifest pluginpkg.Manifest
	if err := yaml.Unmarshal(manifestBytes, &manifest); err != nil {
		fail(err.Error())
	}
	if err := manifest.Validate(sdk.ProtocolVersion); err != nil {
		fail(err.Error())
	}
	if err := os.MkdirAll(*outputDir, 0o750); err != nil {
		fail(err.Error())
	}
	buildDir, err := os.MkdirTemp("", "meerkit-pluginpack-")
	if err != nil {
		fail(err.Error())
	}
	defer os.RemoveAll(buildDir)
	manifest.Artifacts = nil
	for _, item := range targets {
		name := "plugin"
		if item.goos == "windows" {
			name += ".exe"
		}
		path := filepath.Join(buildDir, "bin", item.goos+"-"+item.goarch, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			fail(err.Error())
		}
		command := exec.Command("go", "build", "-trimpath", "-o", path, ".")
		command.Dir = *pluginDir
		command.Env = append(os.Environ(), "GOOS="+item.goos, "GOARCH="+item.goarch, "CGO_ENABLED=0")
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
		if err := command.Run(); err != nil {
			fail(err.Error())
		}
		hash, size, err := hashFile(path)
		if err != nil {
			fail(err.Error())
		}
		manifest.Artifacts = append(manifest.Artifacts, pluginpkg.Artifact{GOOS: item.goos, GOARCH: item.goarch, Path: filepath.ToSlash(strings.TrimPrefix(path, buildDir+string(filepath.Separator))), Size: size, SHA256: hash})
	}
	if *combined {
		output := filepath.Join(*outputDir, fmt.Sprintf("%s-%s-all.zip", manifest.ID, manifest.Version))
		if err := writePackage(output, *pluginDir, buildDir, manifest, manifest.Artifacts, *privateKeyPath, *keyID); err != nil {
			fail(err.Error())
		}
		fmt.Println(output)
		return
	}
	for _, artifact := range manifest.Artifacts {
		extension := ".tar.gz"
		if artifact.GOOS == "windows" {
			extension = ".zip"
		}
		output := filepath.Join(*outputDir, fmt.Sprintf("%s-%s-%s-%s%s", manifest.ID, manifest.Version, artifact.GOOS, artifact.GOARCH, extension))
		if err := writePackage(output, *pluginDir, buildDir, manifest, []pluginpkg.Artifact{artifact}, *privateKeyPath, *keyID); err != nil {
			fail(err.Error())
		}
		fmt.Println(output)
	}
}

func parseTargets(value string) ([]target, error) {
	if value == "current" {
		return []target{{runtime.GOOS, runtime.GOARCH}}, nil
	}
	result := []target{}
	seen := map[string]bool{}
	for _, item := range strings.Split(value, ",") {
		parts := strings.Split(strings.TrimSpace(item), "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid target %q", item)
		}
		key := parts[0] + "/" + parts[1]
		if !seen[key] {
			seen[key] = true
			result = append(result, target{parts[0], parts[1]})
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("at least one target is required")
	}
	return result, nil
}
func writePackage(output, pluginDir, buildDir string, manifest pluginpkg.Manifest, artifacts []pluginpkg.Artifact, keyPath, keyID string) error {
	manifest.Artifacts = artifacts
	if err := manifest.Validate(sdk.ProtocolVersion); err != nil {
		return err
	}
	manifestBytes, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}
	files := map[string][]byte{"meerkit-plugin.yaml": manifestBytes}
	for _, artifact := range artifacts {
		data, err := os.ReadFile(filepath.Join(buildDir, filepath.FromSlash(artifact.Path)))
		if err != nil {
			return err
		}
		files[artifact.Path] = data
	}
	for _, name := range []string{"README.md", "README.en.md", "LICENSE"} {
		if data, err := os.ReadFile(filepath.Join(pluginDir, name)); err == nil {
			files[name] = data
		}
	}
	if keyPath != "" {
		signature, err := sign(pluginpkg.SignaturePayload(manifestBytes, files), keyPath, keyID)
		if err != nil {
			return err
		}
		files["meerkit-plugin.sig"] = signature
	}
	temporary := output + ".tmp"
	_ = os.Remove(temporary)
	if strings.HasSuffix(output, ".zip") {
		err = writeZIP(temporary, files)
	} else {
		err = writeTarGZ(temporary, files)
	}
	if err != nil {
		return err
	}
	return os.Rename(temporary, output)
}
func writeZIP(path string, files map[string][]byte) error {
	output, err := os.Create(path)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(output)
	for name, data := range files {
		header := &zip.FileHeader{Name: filepath.ToSlash(name), Method: zip.Deflate}
		header.SetMode(0o600)
		if strings.HasPrefix(name, "bin/") {
			header.SetMode(0o700)
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			output.Close()
			return err
		}
		if _, err := entry.Write(data); err != nil {
			output.Close()
			return err
		}
	}
	if err := writer.Close(); err != nil {
		output.Close()
		return err
	}
	return output.Close()
}
func writeTarGZ(path string, files map[string][]byte) error {
	output, err := os.Create(path)
	if err != nil {
		return err
	}
	gzipWriter := gzip.NewWriter(output)
	writer := tar.NewWriter(gzipWriter)
	for name, data := range files {
		mode := int64(0o600)
		if strings.HasPrefix(name, "bin/") {
			mode = 0o700
		}
		if err := writer.WriteHeader(&tar.Header{Name: filepath.ToSlash(name), Mode: mode, Size: int64(len(data))}); err != nil {
			output.Close()
			return err
		}
		if _, err := writer.Write(data); err != nil {
			output.Close()
			return err
		}
	}
	if err := writer.Close(); err != nil {
		output.Close()
		return err
	}
	if err := gzipWriter.Close(); err != nil {
		output.Close()
		return err
	}
	return output.Close()
}
func sign(data []byte, path, keyID string) ([]byte, error) {
	if keyID == "" {
		return nil, fmt.Errorf("--key-id is required with --sign-key")
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		return nil, err
	}
	if len(key) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("signing key must contain a base64 Ed25519 private key")
	}
	privateKey := ed25519.PrivateKey(key)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	return json.Marshal(map[string]any{
		"version":    1,
		"key_id":     keyID,
		"public_key": base64.StdEncoding.EncodeToString(publicKey),
		"signature":  base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, data)),
	})
}

func generateKey(prefix string) (string, string, error) {
	if strings.TrimSpace(prefix) == "" {
		return "", "", fmt.Errorf("key path prefix is required")
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	directory := filepath.Dir(prefix)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", "", err
	}
	privatePath := prefix + ".private.key"
	publicPath := prefix + ".public.key"
	if err := writeExclusive(privatePath, []byte(base64.StdEncoding.EncodeToString(privateKey)+"\n"), 0o600); err != nil {
		return "", "", err
	}
	if err := writeExclusive(publicPath, []byte(base64.StdEncoding.EncodeToString(publicKey)+"\n"), 0o644); err != nil {
		_ = os.Remove(privatePath)
		return "", "", err
	}
	return privatePath, publicPath, nil
}

func writeExclusive(path string, data []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func hashFile(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	digest := sha256.New()
	size, err := io.Copy(digest, file)
	return hex.EncodeToString(digest.Sum(nil)), size, err
}
func fail(message string) { fmt.Fprintln(os.Stderr, "pluginpack:", message); os.Exit(1) }
