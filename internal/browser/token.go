package browser

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const pairingTokenFilename = "pairing-token"

func GenerateToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func OpenManager(dataDir string) (*Manager, error) {
	root := filepath.Join(dataDir, "browser")
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, err
	}
	tokenPath := filepath.Join(root, pairingTokenFilename)
	token, err := readOrCreateToken(tokenPath)
	if err != nil {
		return nil, err
	}
	manager, err := NewManager(token)
	if err != nil {
		return nil, err
	}
	manager.tokenPath = tokenPath
	return manager, nil
}

func readOrCreateToken(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		token := strings.TrimSpace(string(data))
		if token == "" {
			return "", errors.New("browser pairing token file is empty")
		}
		return token, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	token, err := GenerateToken()
	if err != nil {
		return "", err
	}
	if err := writeToken(path, token); err != nil {
		return "", err
	}
	return token, nil
}

func writeToken(path, token string) error {
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, []byte(token+"\n"), 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}
