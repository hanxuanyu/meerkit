package core

import (
	"encoding/json"
	"time"
)

type PluginModule struct {
	Type                string `json:"type" yaml:"type"`
	Name                string `json:"name" yaml:"name"`
	Version             string `json:"version" yaml:"version"`
	ConfigVersion       string `json:"config_version" yaml:"config_version"`
	ResultSchemaVersion string `json:"result_schema_version" yaml:"result_schema_version"`
}

type PluginInstallation struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Version           string          `json:"version"`
	Vendor            string          `json:"vendor"`
	Description       string          `json:"desp"`
	URL               string          `json:"url"`
	Enabled           bool            `json:"enabled"`
	Verified          bool            `json:"verified"`
	Official          bool            `json:"official"`
	TrustState        string          `json:"trust_state"`
	SignerKeyID       string          `json:"signer_key_id,omitempty"`
	SignerFingerprint string          `json:"signer_fingerprint,omitempty"`
	Status            string          `json:"status"`
	Error             string          `json:"error,omitempty"`
	PackagePath       string          `json:"-"`
	BinaryPath        string          `json:"-"`
	SignerPublicKey   string          `json:"-"`
	Readme            string          `json:"-"`
	PackageName       string          `json:"package_name"`
	PackageSHA256     string          `json:"package_sha256"`
	Manifest          json.RawMessage `json:"manifest,omitempty"`
	Modules           []PluginModule  `json:"modules"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type PluginDetails struct {
	PluginInstallation
	Readme            string             `json:"readme,omitempty"`
	ModuleDescriptors []ModuleDescriptor `json:"module_descriptors,omitempty"`
}

type TrustedPluginSigner struct {
	Fingerprint string    `json:"fingerprint"`
	KeyID       string    `json:"key_id"`
	PublicKey   string    `json:"public_key"`
	Vendor      string    `json:"vendor"`
	Source      string    `json:"source"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
